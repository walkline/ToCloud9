#include "updates-barrier.h"

#include <spdlog/spdlog.h>

#include <utility>

namespace tc9 {

namespace {
// Same cap as the gateway barrier send loop.
constexpr size_t kMaxUpdatesPerEvent = 1000;
}  // anonymous namespace

CharacterUpdatesBarrier::CharacterUpdatesBarrier(FlushFn flushFn, std::chrono::milliseconds interval)
    : flush_fn_(std::move(flushFn)), interval_(interval) {}

CharacterUpdatesBarrier::~CharacterUpdatesBarrier() {
    Stop();
}

void CharacterUpdatesBarrier::Start() {
    std::lock_guard<std::mutex> lock(mutex_);
    if (running_) {
        return;
    }
    running_ = true;
    thread_ = std::thread(&CharacterUpdatesBarrier::run, this);
}

void CharacterUpdatesBarrier::Stop() {
    {
        std::lock_guard<std::mutex> lock(mutex_);
        if (!running_) {
            return;
        }
        running_ = false;
    }
    cv_.notify_all();
    if (thread_.joinable()) {
        thread_.join();
    }
    // Flush leftovers so updates emitted during shutdown are not lost.
    flush();
}

void CharacterUpdatesBarrier::UpdateZone(uint64_t charGUID, uint32_t mapID, uint32_t areaID, uint32_t zoneID) {
    std::lock_guard<std::mutex> lock(mutex_);
    auto& upd = pending_[charGUID];
    upd.map = mapID;
    upd.area = areaID;
    upd.zone = zoneID;
}

void CharacterUpdatesBarrier::UpdateLevel(uint64_t charGUID, uint8_t level) {
    std::lock_guard<std::mutex> lock(mutex_);
    pending_[charGUID].level = level;
}

void CharacterUpdatesBarrier::run() {
    for (;;) {
        {
            std::unique_lock<std::mutex> lock(mutex_);
            cv_.wait_for(lock, interval_, [this] { return !running_; });
            if (!running_) {
                return;
            }
        }
        flush();
    }
}

void CharacterUpdatesBarrier::flush() {
    std::unordered_map<uint64_t, PendingUpdate> updates;
    {
        std::lock_guard<std::mutex> lock(mutex_);
        updates.swap(pending_);
    }
    if (updates.empty()) {
        return;
    }

    try {
        nlohmann::json batch = nlohmann::json::array();
        for (const auto& [guid, upd] : updates) {
            nlohmann::json item = {{"i", guid}};
            if (upd.level) {
                item["l"] = *upd.level;
            }
            if (upd.map) {
                item["m"] = *upd.map;
            }
            if (upd.area) {
                item["a"] = *upd.area;
            }
            if (upd.zone) {
                item["z"] = *upd.zone;
            }
            batch.push_back(std::move(item));

            if (batch.size() >= kMaxUpdatesPerEvent) {
                flush_fn_(std::move(batch));
                batch = nlohmann::json::array();
            }
        }
        if (!batch.empty()) {
            flush_fn_(std::move(batch));
        }
    } catch (const std::exception& e) {
        spdlog::error("Error flushing character updates: {}", e.what());
    }
}

}  // namespace tc9
