#include <gtest/gtest.h>
#include "../src/guids/guid_manager.h"
#include <thread>
#include <vector>
#include <set>
#include <atomic>
#include <chrono>

using namespace tc9;

// Mock GrpcClients for testing
class MockGrpcClients {
public:
    static std::vector<std::pair<uint64_t, uint64_t>> GenerateRanges(
        int range_count, uint64_t range_size, uint64_t& counter) {
        std::vector<std::pair<uint64_t, uint64_t>> ranges;

        for (int i = 0; i < range_count; ++i) {
            uint64_t start = counter + 1;
            counter += range_size;
            uint64_t end = counter;
            ranges.push_back({start, end});
        }

        return ranges;
    }
};

// Test GuidIterator with single range
TEST(GuidIteratorTest, SingleRange_Sequential) {
    GuidIterator iter(0, false);  // Character type, non-thread-safe

    // Add single range [1, 11)
    std::vector<std::pair<uint64_t, uint64_t>> ranges = {{1, 11}};
    iter.AddRanges(ranges);

    // Should generate 1, 2, 3, ..., 10
    for (uint64_t i = 1; i <= 10; ++i) {
        EXPECT_EQ(i, iter.Next(1));
    }

    // After exhausting range, should return 0 (error)
    EXPECT_EQ(0, iter.Next(1));
}

// Test GuidIterator with multiple ranges
TEST(GuidIteratorTest, MultipleRanges_Sequential) {
    GuidIterator iter(0, false);

    // Add two ranges: [1, 6) and [6, 11)
    std::vector<std::pair<uint64_t, uint64_t>> ranges = {
        {1, 6},
        {6, 11}
    };
    iter.AddRanges(ranges);

    // Should generate 1, 2, 3, 4, 5, 6, 7, 8, 9, 10
    for (uint64_t i = 1; i <= 10; ++i) {
        EXPECT_EQ(i, iter.Next(1));
    }
}

// Test GuidIterator with smallest ranges (size 1)
TEST(GuidIteratorTest, SmallestRanges) {
    GuidIterator iter(0, false);

    // Add ranges of size 1: [1, 2), [2, 3), [3, 4), ...
    std::vector<std::pair<uint64_t, uint64_t>> ranges;
    for (uint64_t i = 1; i <= 10; ++i) {
        ranges.push_back({i, i + 1});
    }
    iter.AddRanges(ranges);

    // Should generate 1, 2, 3, ..., 10
    for (uint64_t i = 1; i <= 10; ++i) {
        EXPECT_EQ(i, iter.Next(1));
    }
}

// Test GuidIterator with range size 2
TEST(GuidIteratorTest, RangeSize2) {
    GuidIterator iter(0, false);

    // Add ranges: [1, 3), [3, 5), [5, 7), ...
    std::vector<std::pair<uint64_t, uint64_t>> ranges;
    for (uint64_t i = 1; i <= 9; i += 2) {
        ranges.push_back({i, i + 2});
    }
    iter.AddRanges(ranges);

    // Should generate 1, 2, 3, ..., 10
    for (uint64_t i = 1; i <= 10; ++i) {
        EXPECT_EQ(i, iter.Next(1));
    }
}

// Test thread-safe iterator (Items)
TEST(GuidIteratorTest, ThreadSafe_Sequential) {
    GuidIterator iter(1, true);  // Item type, thread-safe

    // Add range [1, 101)
    std::vector<std::pair<uint64_t, uint64_t>> ranges = {{1, 101}};
    iter.AddRanges(ranges);

    // Should generate sequential GUIDs even with atomics
    for (uint64_t i = 1; i <= 100; ++i) {
        EXPECT_EQ(i, iter.Next(1));
    }
}

// Test thread-safe iterator under concurrent access
TEST(GuidIteratorTest, ThreadSafe_Concurrent) {
    GuidIterator iter(1, true);  // Item type, thread-safe

    // Add large range [1, 10001)
    std::vector<std::pair<uint64_t, uint64_t>> ranges = {{1, 10001}};
    iter.AddRanges(ranges);

    const int num_threads = 10;
    const int guids_per_thread = 1000;

    std::vector<std::thread> threads;
    std::vector<std::vector<uint64_t>> thread_guids(num_threads);

    // Spawn threads to generate GUIDs concurrently
    for (int t = 0; t < num_threads; ++t) {
        threads.emplace_back([&iter, &thread_guids, t, guids_per_thread]() {
            for (int i = 0; i < guids_per_thread; ++i) {
                uint64_t guid = iter.Next(1);
                thread_guids[t].push_back(guid);
            }
        });
    }

    // Wait for all threads
    for (auto& thread : threads) {
        thread.join();
    }

    // Verify: no duplicates, all GUIDs in range [1, 10000]
    std::set<uint64_t> all_guids;
    for (const auto& guids : thread_guids) {
        for (uint64_t guid : guids) {
            EXPECT_TRUE(guid >= 1 && guid <= 10000) << "GUID out of range: " << guid;
            EXPECT_TRUE(all_guids.insert(guid).second) << "Duplicate GUID: " << guid;
        }
    }

    // Should have exactly 10000 unique GUIDs
    EXPECT_EQ(10000, all_guids.size());
}

// Test available count
TEST(GuidIteratorTest, AvailableCount) {
    GuidIterator iter(0, false);

    // Add ranges totaling 100 GUIDs
    std::vector<std::pair<uint64_t, uint64_t>> ranges = {
        {1, 51},    // 50 GUIDs
        {51, 101}   // 50 GUIDs
    };
    iter.AddRanges(ranges);

    EXPECT_EQ(100, iter.AvailableCount());

    // Consume 10 GUIDs
    for (int i = 0; i < 10; ++i) {
        iter.Next(1);
    }

    EXPECT_EQ(90, iter.AvailableCount());
}

// Test needs refill threshold
TEST(GuidIteratorTest, NeedsRefill) {
    GuidIterator iter(0, false);

    // Add small range that will trigger refill
    std::vector<std::pair<uint64_t, uint64_t>> ranges = {{1, 500}};
    iter.AddRanges(ranges);

    // Should not need refill initially (499 available > 1000 threshold is false, but < 1000)
    // Actually, since 499 < 1000, it should need refill
    EXPECT_TRUE(iter.NeedsRefill());

    // Add more ranges
    ranges = {{500, 2000}};
    iter.AddRanges(ranges);

    // Now should not need refill (1999 available > 1000)
    EXPECT_FALSE(iter.NeedsRefill());
}

// Performance test: Non-thread-safe iterator
TEST(GuidIteratorTest, Performance_NonThreadSafe) {
    GuidIterator iter(0, false);

    // Add large range
    std::vector<std::pair<uint64_t, uint64_t>> ranges = {{1, 100001}};
    iter.AddRanges(ranges);

    auto start = std::chrono::high_resolution_clock::now();

    for (int i = 0; i < 100000; ++i) {
        iter.Next(1);
    }

    auto end = std::chrono::high_resolution_clock::now();
    auto duration = std::chrono::duration_cast<std::chrono::nanoseconds>(end - start);

    double ns_per_call = static_cast<double>(duration.count()) / 100000.0;

    std::cout << "Non-thread-safe iterator: " << ns_per_call << " ns/call" << std::endl;

    // Should be < 50ns per call (typically ~10ns)
    EXPECT_LT(ns_per_call, 50.0);
}

// Performance test: Thread-safe iterator
TEST(GuidIteratorTest, Performance_ThreadSafe) {
    GuidIterator iter(1, true);

    // Add large range
    std::vector<std::pair<uint64_t, uint64_t>> ranges = {{1, 100001}};
    iter.AddRanges(ranges);

    auto start = std::chrono::high_resolution_clock::now();

    for (int i = 0; i < 100000; ++i) {
        iter.Next(1);
    }

    auto end = std::chrono::high_resolution_clock::now();
    auto duration = std::chrono::duration_cast<std::chrono::nanoseconds>(end - start);

    double ns_per_call = static_cast<double>(duration.count()) / 100000.0;

    std::cout << "Thread-safe iterator: " << ns_per_call << " ns/call" << std::endl;

    // Should be < 100ns per call (typically ~50ns due to atomics)
    EXPECT_LT(ns_per_call, 100.0);
}

// Stress test: Concurrent access with collision detection
TEST(GuidIteratorTest, StressTest_Concurrent) {
    GuidIterator iter(1, true);

    // Add very large range
    std::vector<std::pair<uint64_t, uint64_t>> ranges = {{1, 50001}};
    iter.AddRanges(ranges);

    const int num_threads = 20;
    const int guids_per_thread = 2500;

    std::atomic<int> collision_count{0};
    std::vector<std::thread> threads;
    std::vector<std::set<uint64_t>> thread_guids(num_threads);

    for (int t = 0; t < num_threads; ++t) {
        threads.emplace_back([&iter, &thread_guids, &collision_count, t, guids_per_thread]() {
            for (int i = 0; i < guids_per_thread; ++i) {
                uint64_t guid = iter.Next(1);

                if (!thread_guids[t].insert(guid).second) {
                    collision_count.fetch_add(1);
                }
            }
        });
    }

    for (auto& thread : threads) {
        thread.join();
    }

    // Combine all GUIDs and check for cross-thread collisions
    std::set<uint64_t> all_guids;
    for (const auto& guids : thread_guids) {
        for (uint64_t guid : guids) {
            EXPECT_TRUE(all_guids.insert(guid).second) << "Cross-thread collision detected!";
        }
    }

    EXPECT_EQ(0, collision_count.load()) << "Intra-thread collisions detected!";
    EXPECT_EQ(num_threads * guids_per_thread, all_guids.size());
}

// Test GuidIterator with incremental range additions
TEST(GuidIteratorTest, IncrementalRangeAddition) {
    GuidIterator iter(0, false);

    // Start with small range
    std::vector<std::pair<uint64_t, uint64_t>> ranges = {{1, 6}};
    iter.AddRanges(ranges);

    // Generate first 5
    for (uint64_t i = 1; i <= 5; ++i) {
        EXPECT_EQ(i, iter.Next(1));
    }

    // Add more ranges before exhausting current one
    ranges = {{6, 11}};
    iter.AddRanges(ranges);

    // Should continue seamlessly
    for (uint64_t i = 6; i <= 10; ++i) {
        EXPECT_EQ(i, iter.Next(1));
    }
}
