#ifndef TC9_GUID_MANAGER_H
#define TC9_GUID_MANAGER_H

#include <cstdint>
#include <vector>
#include <atomic>
#include <mutex>
#include <memory>

namespace tc9 {

// Forward declarations
class GrpcClients;

// Represents a range of GUIDs [start, end)
struct GuidRange {
    uint64_t start;
    uint64_t end;

    GuidRange(uint64_t s, uint64_t e) : start(s), end(e) {}

    size_t size() const { return static_cast<size_t>(end - start); }
    bool contains(uint64_t guid) const { return guid >= start && guid < end; }
};

// Iterator for generating sequential GUIDs from pre-fetched ranges
class GuidIterator {
public:
    explicit GuidIterator(int guid_type, bool thread_safe = false);
    ~GuidIterator() = default;

    // Get next GUID (thread-safe if configured)
    uint64_t Next(uint32_t realm_id);

    // Add new ranges (from GUID service)
    void AddRanges(const std::vector<std::pair<uint64_t, uint64_t>>& ranges);

    // Check if we need to request more GUIDs
    bool NeedsRefill() const;

    // Get total available GUIDs
    size_t AvailableCount() const;

private:
    void MoveToNextRange();

    int guid_type_;                     // 0=Character, 1=Item, 2=Instance
    bool thread_safe_;                  // Use atomics if true

    // GUID ranges and current position
    std::vector<GuidRange> ranges_;
    size_t current_range_idx_ = 0;

    // Thread-safe: atomic counter
    std::atomic<uint64_t> current_guid_atomic_{0};
    std::atomic<uint64_t> current_range_end_atomic_{0};

    // Non-thread-safe: regular counter (faster)
    uint64_t current_guid_ = 0;
    uint64_t current_range_end_ = 0;

    // Mutex only for range management (rare operations)
    mutable std::mutex ranges_mutex_;

    static constexpr size_t REFILL_THRESHOLD = 1000;
};

// Manages GUID generation for all types
class GuidManager {
public:
    static GuidManager& Instance();

    // Initialize with gRPC client for fetching ranges
    void Initialize(GrpcClients* clients, uint32_t realm_id);

    // Get next GUID (realmID=0 uses default realm)
    uint64_t GetNextCharacterGuid(uint32_t realm_id);
    uint64_t GetNextItemGuid(uint32_t realm_id);        // THREAD-SAFE
    uint64_t GetNextInstanceGuid(uint32_t realm_id);

    // Check if initialization is needed
    bool IsInitialized() const { return initialized_; }

    // Delete copy/move
    GuidManager(const GuidManager&) = delete;
    GuidManager& operator=(const GuidManager&) = delete;
    GuidManager(GuidManager&&) = delete;
    GuidManager& operator=(GuidManager&&) = delete;

private:
    GuidManager();
    ~GuidManager() = default;

    // Request more GUIDs from service (async)
    void RefillGuidPool(int guid_type, uint32_t realm_id);

    GrpcClients* grpc_clients_ = nullptr;
    uint32_t default_realm_id_ = 0;
    bool initialized_ = false;

    // Three separate iterators for each GUID type
    std::unique_ptr<GuidIterator> character_guids_;  // Thread-unsafe (fast)
    std::unique_ptr<GuidIterator> item_guids_;       // Thread-safe (atomics)
    std::unique_ptr<GuidIterator> instance_guids_;   // Thread-unsafe (fast)

    // Pool size to request from service
    static constexpr uint64_t POOL_SIZE = 10000;
};

}  // namespace tc9

#endif  // TC9_GUID_MANAGER_H
