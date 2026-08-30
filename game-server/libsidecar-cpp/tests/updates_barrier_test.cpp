#include <gtest/gtest.h>
#include "../src/events/updates-barrier.h"

#include <chrono>
#include <vector>

using namespace tc9;

namespace {

// An interval long enough that the flush thread never fires on its own:
// every flush observed by these tests is deterministic (driven by Stop()).
constexpr std::chrono::milliseconds kNeverFires{3600000};

struct CapturingFlush {
    std::vector<nlohmann::json> batches;

    CharacterUpdatesBarrier::FlushFn fn() {
        return [this](nlohmann::json updates) { batches.push_back(std::move(updates)); };
    }
};

}  // namespace

TEST(CharacterUpdatesBarrierTest, MergesUpdatesPerCharacter) {
    CapturingFlush captured;
    CharacterUpdatesBarrier barrier(captured.fn(), kNeverFires);
    barrier.Start();

    barrier.UpdateZone(42, 1, 12, 33);
    barrier.UpdateLevel(42, 80);
    barrier.UpdateZone(42, 0, 14, 40);  // newer zone overwrites the pending one
    barrier.UpdateLevel(7, 10);

    barrier.Stop();

    ASSERT_EQ(captured.batches.size(), 1u);
    const auto& batch = captured.batches[0];
    ASSERT_EQ(batch.size(), 2u);

    for (const auto& item : batch) {
        if (item["i"] == 42) {
            EXPECT_EQ(item["l"], 80);
            EXPECT_EQ(item["m"], 0);
            EXPECT_EQ(item["a"], 14);
            EXPECT_EQ(item["z"], 40);
        } else {
            EXPECT_EQ(item["i"], 7);
            EXPECT_EQ(item["l"], 10);
            EXPECT_FALSE(item.contains("z"));
        }
    }
}

TEST(CharacterUpdatesBarrierTest, SplitsBatchesAtTheCap) {
    CapturingFlush captured;
    CharacterUpdatesBarrier barrier(captured.fn(), kNeverFires);
    barrier.Start();

    for (uint64_t g = 1; g <= 1500; ++g) {
        barrier.UpdateLevel(g, 5);
    }

    barrier.Stop();

    ASSERT_EQ(captured.batches.size(), 2u);
    EXPECT_EQ(captured.batches[0].size() + captured.batches[1].size(), 1500u);
    EXPECT_EQ(captured.batches[0].size(), 1000u);
}

TEST(CharacterUpdatesBarrierTest, StopFlushesWithoutStart) {
    CapturingFlush captured;
    CharacterUpdatesBarrier barrier(captured.fn(), kNeverFires);

    // Never started: updates queued before/without Start must still be
    // delivered by Stop() (also covers the destructor path).
    barrier.UpdateLevel(1, 3);
    barrier.Stop();

    ASSERT_EQ(captured.batches.size(), 1u);
    EXPECT_EQ(captured.batches[0].size(), 1u);
}

TEST(CharacterUpdatesBarrierTest, StopIsIdempotentAndFlushesLateUpdates) {
    CapturingFlush captured;
    CharacterUpdatesBarrier barrier(captured.fn(), kNeverFires);
    barrier.Start();

    barrier.UpdateLevel(1, 3);
    barrier.Stop();
    ASSERT_EQ(captured.batches.size(), 1u);

    // An update racing the shutdown lands after the first Stop: the second
    // Stop (e.g. the destructor) must deliver it rather than drop it.
    barrier.UpdateLevel(2, 4);
    barrier.Stop();
    ASSERT_EQ(captured.batches.size(), 2u);
    EXPECT_EQ(captured.batches[1][0]["i"], 2);
}
