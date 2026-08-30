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
    bool was_running = false;
    {
        std::lock_guard<std::mutex> lock(mutex_);
        was_running = running_;
        running_ = false;
    }
    if (was_running) {
        cv_.notify_all();
        if (thread_.joinable()) {
            thread_.join();
        }
    }
    // Flush leftovers unconditionally: updates can be queued while the
    // barrier is stopped (or before Start) and must not be lost.
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

void CharacterUpdatesBarrier::UpdateVitals(uint64_t charGUID, uint8_t level, uint32_t curHP, uint32_t maxHP,
                                           uint8_t powerType, uint32_t curPower, uint32_t maxPower,
                                           float posX, float posY, bool isDead, bool isGhost) {
    std::lock_guard<std::mutex> lock(mutex_);
    auto& upd = pending_[charGUID];
    upd.level = level;
    upd.curHP = curHP;
    upd.maxHP = maxHP;
    upd.powerType = powerType;
    upd.curPower = curPower;
    upd.maxPower = maxPower;
    upd.posX = posX;
    upd.posY = posY;
    upd.isDead = isDead;
    upd.isGhost = isGhost;
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
            if (upd.curHP) {
                item["h"] = *upd.curHP;
            }
            if (upd.maxHP) {
                item["hm"] = *upd.maxHP;
            }
            if (upd.powerType) {
                item["pt"] = *upd.powerType;
            }
            if (upd.curPower) {
                item["p"] = *upd.curPower;
            }
            if (upd.maxPower) {
                item["pm"] = *upd.maxPower;
            }
            if (upd.posX) {
                item["x"] = *upd.posX;
            }
            if (upd.posY) {
                item["y"] = *upd.posY;
            }
            if (upd.isDead) {
                item["d"] = *upd.isDead;
            }
            if (upd.isGhost) {
                item["g"] = *upd.isGhost;
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
