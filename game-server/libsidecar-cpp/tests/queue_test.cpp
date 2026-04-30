#include <gtest/gtest.h>
#include "../src/queue/handlers_queue.h"
#include <thread>

using namespace tc9;

TEST(HandlersQueueTest, PushAndPop) {
    HandlersQueue queue;

    int value = 0;
    queue.Push(MakeHandler([&value]() { value = 42; }));

    EXPECT_FALSE(queue.Empty());
    EXPECT_EQ(queue.Size(), 1);

    auto handler = queue.Pop();
    ASSERT_NE(handler, nullptr);
    handler->Handle();

    EXPECT_EQ(value, 42);
    EXPECT_TRUE(queue.Empty());
}

TEST(HandlersQueueTest, PopEmptyQueue) {
    HandlersQueue queue;

    auto handler = queue.Pop();
    EXPECT_EQ(handler, nullptr);
}

TEST(HandlersQueueTest, MultipleHandlers) {
    HandlersQueue queue;

    std::vector<int> results;

    queue.Push(MakeHandler([&results]() { results.push_back(1); }));
    queue.Push(MakeHandler([&results]() { results.push_back(2); }));
    queue.Push(MakeHandler([&results]() { results.push_back(3); }));

    EXPECT_EQ(queue.Size(), 3);

    while (auto handler = queue.Pop()) {
        handler->Handle();
    }

    ASSERT_EQ(results.size(), 3);
    EXPECT_EQ(results[0], 1);
    EXPECT_EQ(results[1], 2);
    EXPECT_EQ(results[2], 3);
}

TEST(HandlersQueueTest, ThreadSafety) {
    HandlersQueue queue;
    std::atomic<int> counter{0};

    constexpr int num_threads = 4;
    constexpr int items_per_thread = 1000;

    // Push from multiple threads
    std::vector<std::thread> push_threads;
    for (int i = 0; i < num_threads; ++i) {
        push_threads.emplace_back([&queue, &counter]() {
            for (int j = 0; j < items_per_thread; ++j) {
                queue.Push(MakeHandler([&counter]() { counter++; }));
            }
        });
    }

    for (auto& t : push_threads) {
        t.join();
    }

    EXPECT_EQ(queue.Size(), num_threads * items_per_thread);

    // Pop and execute all
    while (auto handler = queue.Pop()) {
        handler->Handle();
    }

    EXPECT_EQ(counter, num_threads * items_per_thread);
}
