#ifndef TC9_EVENTS_UPDATES_BARRIER_H
#define TC9_EVENTS_UPDATES_BARRIER_H

#include <chrono>
#include <condition_variable>
#include <cstdint>
#include <functional>
#include <mutex>
#include <optional>
#include <thread>
#include <unordered_map>

#include <nlohmann/json.hpp>

namespace tc9 {

// Batches per-character field updates and flushes them periodically,
// mirroring the gateway CharactersUpdatesBarrier
// (apps/gateway/service/characters-updates-barrier.go). Updates for the
// same character merge while the barrier is closed, so consumers see at
// most one entry per character per flush.
class CharacterUpdatesBarrier {
public:
    // Receives the merged updates as a JSON array of
    // shared/events.CharacterUpdate objects ("i"/"l"/"m"/"a"/"z" keys).
    using FlushFn = std::function<void(nlohmann::json updates)>;

    CharacterUpdatesBarrier(FlushFn flushFn, std::chrono::milliseconds interval);
    ~CharacterUpdatesBarrier();

    void Start();
    void Stop();

    // Thread-safe.
    void UpdateZone(uint64_t charGUID, uint32_t mapID, uint32_t areaID, uint32_t zoneID);
    void UpdateLevel(uint64_t charGUID, uint8_t level);

    // Delete copy/move
    CharacterUpdatesBarrier(const CharacterUpdatesBarrier&) = delete;
    CharacterUpdatesBarrier& operator=(const CharacterUpdatesBarrier&) = delete;
    CharacterUpdatesBarrier(CharacterUpdatesBarrier&&) = delete;
    CharacterUpdatesBarrier& operator=(CharacterUpdatesBarrier&&) = delete;

private:
    struct PendingUpdate {
        std::optional<uint8_t> level;
        std::optional<uint32_t> map;
        std::optional<uint32_t> area;
        std::optional<uint32_t> zone;
    };

    void run();
    void flush();

    FlushFn flush_fn_;
    std::chrono::milliseconds interval_;

    std::mutex mutex_;
    std::condition_variable cv_;
    std::unordered_map<uint64_t, PendingUpdate> pending_;
    std::thread thread_;
    bool running_ = false;
};

}  // namespace tc9

#endif  // TC9_EVENTS_UPDATES_BARRIER_H
