#include "guid_manager.h"
#include "../grpc/clients.h"
#include <spdlog/spdlog.h>
#include <thread>

namespace tc9 {

// GuidIterator Implementation

GuidIterator::GuidIterator(int guid_type, bool thread_safe)
    : guid_type_(guid_type), thread_safe_(thread_safe) {
    spdlog::debug("GuidIterator created for type {} (thread_safe={})", guid_type, thread_safe);
}

uint64_t GuidIterator::Next(uint32_t realm_id) {
    if (thread_safe_) {
        // Thread-safe path: use atomics
        uint64_t guid = current_guid_atomic_.fetch_add(1, std::memory_order_relaxed);
        uint64_t range_end = current_range_end_atomic_.load(std::memory_order_relaxed);

        // Check if we've exhausted current range
        if (guid >= range_end) {
            std::lock_guard<std::mutex> lock(ranges_mutex_);

            // Double-check after acquiring lock (another thread may have moved)
            guid = current_guid_atomic_.load(std::memory_order_relaxed);
            range_end = current_range_end_atomic_.load(std::memory_order_relaxed);

            if (guid >= range_end) {
                MoveToNextRange();
                guid = current_guid_atomic_.fetch_add(1, std::memory_order_relaxed);
            }
        }

        return guid;

    } else {
        // Non-thread-safe path: faster, no atomics
        uint64_t guid = current_guid_++;

        if (guid >= current_range_end_) {
            std::lock_guard<std::mutex> lock(ranges_mutex_);
            MoveToNextRange();
            guid = current_guid_++;
        }

        return guid;
    }
}

void GuidIterator::AddRanges(const std::vector<std::pair<uint64_t, uint64_t>>& new_ranges) {
    std::lock_guard<std::mutex> lock(ranges_mutex_);

    size_t old_size = ranges_.size();

    for (const auto& [start, end] : new_ranges) {
        if (end > start) {
            ranges_.emplace_back(start, end);
        }
    }

    spdlog::info("GuidIterator type {}: added {} ranges ({} total, {} available GUIDs)",
                 guid_type_, new_ranges.size(), ranges_.size(), AvailableCount());

    // If this is the first range, initialize counters
    if (old_size == 0 && !ranges_.empty()) {
        current_range_idx_ = 0;
        if (thread_safe_) {
            current_guid_atomic_.store(ranges_[0].start, std::memory_order_relaxed);
            current_range_end_atomic_.store(ranges_[0].end, std::memory_order_relaxed);
        } else {
            current_guid_ = ranges_[0].start;
            current_range_end_ = ranges_[0].end;
        }
    }
}

bool GuidIterator::NeedsRefill() const {
    std::lock_guard<std::mutex> lock(ranges_mutex_);
    return AvailableCount() < REFILL_THRESHOLD;
}

size_t GuidIterator::AvailableCount() const {
    // Must be called with lock held
    size_t count = 0;

    uint64_t current = thread_safe_
        ? current_guid_atomic_.load(std::memory_order_relaxed)
        : current_guid_;

    // Count remaining in current range
    if (current_range_idx_ < ranges_.size()) {
        uint64_t range_end = ranges_[current_range_idx_].end;
        if (current < range_end) {
            count += static_cast<size_t>(range_end - current);
        }
    }

    // Count all remaining ranges
    for (size_t i = current_range_idx_ + 1; i < ranges_.size(); ++i) {
        count += ranges_[i].size();
    }

    return count;
}

void GuidIterator::MoveToNextRange() {
    // Must be called with lock held
    current_range_idx_++;

    if (current_range_idx_ < ranges_.size()) {
        const auto& range = ranges_[current_range_idx_];

        if (thread_safe_) {
            current_guid_atomic_.store(range.start, std::memory_order_relaxed);
            current_range_end_atomic_.store(range.end, std::memory_order_relaxed);
        } else {
            current_guid_ = range.start;
            current_range_end_ = range.end;
        }

        spdlog::debug("GuidIterator type {}: moved to range {} ({} - {})",
                     guid_type_, current_range_idx_, range.start, range.end);
    } else {
        spdlog::error("GuidIterator type {}: exhausted all ranges!", guid_type_);

        // Set to 0 to signal error
        if (thread_safe_) {
            current_guid_atomic_.store(0, std::memory_order_relaxed);
            current_range_end_atomic_.store(0, std::memory_order_relaxed);
        } else {
            current_guid_ = 0;
            current_range_end_ = 0;
        }
    }
}

// GuidManager Implementation

GuidManager& GuidManager::Instance() {
    static GuidManager instance;
    return instance;
}

GuidManager::GuidManager() {
    // Create iterators (items are thread-safe, others are not)
    character_guids_ = std::make_unique<GuidIterator>(0, false);  // Character
    item_guids_ = std::make_unique<GuidIterator>(1, true);        // Item (THREAD-SAFE)
    instance_guids_ = std::make_unique<GuidIterator>(2, false);   // Instance

    spdlog::debug("GuidManager created");
}

void GuidManager::Initialize(GrpcClients* clients, uint32_t realm_id) {
    if (initialized_) {
        spdlog::warn("GuidManager already initialized");
        return;
    }

    if (!clients) {
        spdlog::error("Cannot initialize GuidManager: null GrpcClients");
        return;
    }

    grpc_clients_ = clients;
    default_realm_id_ = realm_id;

    spdlog::info("GuidManager initialized for realm {}", realm_id);

    // Pre-fetch initial GUID pools for all types
    RefillGuidPool(0, realm_id);  // Characters
    RefillGuidPool(1, realm_id);  // Items
    RefillGuidPool(2, realm_id);  // Instances

    initialized_ = true;
}

uint64_t GuidManager::GetNextCharacterGuid(uint32_t realm_id) {
    if (!initialized_) {
        spdlog::error("GuidManager not initialized");
        return 0;
    }

    uint32_t realm = realm_id ? realm_id : default_realm_id_;
    uint64_t guid = character_guids_->Next(realm);

    // Check if we need to refill (async)
    if (character_guids_->NeedsRefill()) {
        std::thread([this, realm]() {
            RefillGuidPool(0, realm);
        }).detach();
    }

    return guid;
}

uint64_t GuidManager::GetNextItemGuid(uint32_t realm_id) {
    if (!initialized_) {
        spdlog::error("GuidManager not initialized");
        return 0;
    }

    uint32_t realm = realm_id ? realm_id : default_realm_id_;
    uint64_t guid = item_guids_->Next(realm);

    // Check if we need to refill (async)
    if (item_guids_->NeedsRefill()) {
        std::thread([this, realm]() {
            RefillGuidPool(1, realm);
        }).detach();
    }

    return guid;
}

uint64_t GuidManager::GetNextInstanceGuid(uint32_t realm_id) {
    if (!initialized_) {
        spdlog::error("GuidManager not initialized");
        return 0;
    }

    uint32_t realm = realm_id ? realm_id : default_realm_id_;
    uint64_t guid = instance_guids_->Next(realm);

    // Check if we need to refill (async)
    if (instance_guids_->NeedsRefill()) {
        std::thread([this, realm]() {
            RefillGuidPool(2, realm);
        }).detach();
    }

    return guid;
}

void GuidManager::RefillGuidPool(int guid_type, uint32_t realm_id) {
    if (!grpc_clients_) {
        spdlog::error("Cannot refill GUID pool: no gRPC client");
        return;
    }

    spdlog::info("Requesting GUID pool for type {} (realm {})", guid_type, realm_id);

    std::vector<std::pair<uint64_t, uint64_t>> ranges;
    bool success = grpc_clients_->RequestGUIDPool(
        realm_id,
        guid_type,
        POOL_SIZE,
        ranges
    );

    if (success && !ranges.empty()) {
        // Add ranges to appropriate iterator
        switch (guid_type) {
            case 0:  // Character
                character_guids_->AddRanges(ranges);
                break;
            case 1:  // Item
                item_guids_->AddRanges(ranges);
                break;
            case 2:  // Instance
                instance_guids_->AddRanges(ranges);
                break;
            default:
                spdlog::error("Invalid GUID type: {}", guid_type);
        }
    } else {
        spdlog::error("Failed to fetch GUID pool for type {}", guid_type);
    }
}

}  // namespace tc9
