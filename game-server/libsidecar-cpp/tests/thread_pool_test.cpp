#include <gtest/gtest.h>
#include "../src/core/thread_pool.h"
#include <atomic>

using namespace tc9;

TEST(ThreadPoolTest, BasicExecution) {
    ThreadPool pool(4);

    std::atomic<int> counter{0};

    pool.Submit([&counter]() { counter++; });
    pool.Submit([&counter]() { counter++; });
    pool.Submit([&counter]() { counter++; });

    // Give time for execution
    std::this_thread::sleep_for(std::chrono::milliseconds(100));

    EXPECT_EQ(counter, 3);
}

TEST(ThreadPoolTest, ExecuteAll) {
    ThreadPool pool(4);

    std::atomic<int> counter{0};

    std::vector<std::function<void()>> tasks;
    for (int i = 0; i < 100; ++i) {
        tasks.push_back([&counter]() { counter++; });
    }

    pool.ExecuteAll(tasks);

    EXPECT_EQ(counter, 100);
}

TEST(ThreadPoolTest, ParallelExecution) {
    ThreadPool pool(4);

    std::atomic<int> max_concurrent{0};
    std::atomic<int> current_concurrent{0};

    std::vector<std::function<void()>> tasks;
    for (int i = 0; i < 10; ++i) {
        tasks.push_back([&]() {
            int current = ++current_concurrent;

            // Update max
            int expected = max_concurrent.load();
            while (current > expected &&
                   !max_concurrent.compare_exchange_weak(expected, current)) {
                expected = max_concurrent.load();
            }

            std::this_thread::sleep_for(std::chrono::milliseconds(10));
            --current_concurrent;
        });
    }

    pool.ExecuteAll(tasks);

    // Should have had at least 2 tasks running concurrently
    EXPECT_GE(max_concurrent, 2);
}

TEST(ThreadPoolTest, ExceptionHandling) {
    ThreadPool pool(2);

    std::atomic<bool> task1_executed{false};
    std::atomic<bool> task2_executed{false};

    std::vector<std::function<void()>> tasks;

    tasks.push_back([&]() {
        task1_executed = true;
        throw std::runtime_error("Test exception");
    });

    tasks.push_back([&]() {
        task2_executed = true;
    });

    // ExecuteAll should handle exceptions gracefully
    EXPECT_NO_THROW(pool.ExecuteAll(tasks));

    EXPECT_TRUE(task1_executed);
    EXPECT_TRUE(task2_executed);
}
