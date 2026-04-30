#include <gtest/gtest.h>
#include "../src/core/thread_pool.h"
#include "../src/queue/handlers_queue.h"
#include <chrono>
#include <vector>
#include <atomic>
#include <thread>

using namespace tc9;
using namespace std::chrono;

// Simulate a read operation with configurable duration
class SimulatedReadHandler : public Handler {
public:
    explicit SimulatedReadHandler(microseconds duration, std::atomic<int>* counter = nullptr)
        : duration_(duration), counter_(counter) {}

    void Handle() override {
        // Simulate work (e.g., memory lookup, data processing)
        std::this_thread::sleep_for(duration_);
        if (counter_) {
            counter_->fetch_add(1);
        }
    }

private:
    microseconds duration_;
    std::atomic<int>* counter_;
};

// Benchmark: Sequential vs Parallel processing
TEST(RequestProcessingBenchmark, SequentialVsParallel_10Requests) {
    const int NUM_REQUESTS = 10;
    const auto REQUEST_DURATION = microseconds(1000);  // 1ms per request
    const int NUM_THREADS = 4;

    // Sequential processing
    {
        std::atomic<int> counter{0};
        HandlersQueue queue;

        // Queue up requests
        for (int i = 0; i < NUM_REQUESTS; ++i) {
            queue.Push(std::make_unique<SimulatedReadHandler>(REQUEST_DURATION, &counter));
        }

        auto start = high_resolution_clock::now();

        // Process sequentially
        while (auto handler = queue.Pop()) {
            handler->Handle();
        }

        auto end = high_resolution_clock::now();
        auto duration = duration_cast<milliseconds>(end - start);

        EXPECT_EQ(NUM_REQUESTS, counter.load());
        std::cout << "Sequential (" << NUM_REQUESTS << " requests): "
                  << duration.count() << " ms" << std::endl;

        // Should take approximately NUM_REQUESTS * 1ms = 10ms
        EXPECT_GE(duration.count(), 9);  // At least 9ms
        EXPECT_LE(duration.count(), 15); // At most 15ms (with overhead)
    }

    // Parallel processing
    {
        std::atomic<int> counter{0};
        HandlersQueue queue;
        ThreadPool pool(NUM_THREADS);

        // Queue up requests
        for (int i = 0; i < NUM_REQUESTS; ++i) {
            queue.Push(std::make_unique<SimulatedReadHandler>(REQUEST_DURATION, &counter));
        }

        auto start = high_resolution_clock::now();

        // Process in parallel (matching TC9ProcessGRPCOrHTTPRequests implementation)
        std::vector<std::function<void()>> tasks;
        while (auto handler = queue.Pop()) {
            auto h = std::shared_ptr<Handler>(std::move(handler));
            tasks.push_back([h]() { h->Handle(); });
        }

        pool.ExecuteAll(tasks);

        auto end = high_resolution_clock::now();
        auto duration = duration_cast<milliseconds>(end - start);

        EXPECT_EQ(NUM_REQUESTS, counter.load());
        std::cout << "Parallel   (" << NUM_REQUESTS << " requests, " << NUM_THREADS
                  << " threads): " << duration.count() << " ms" << std::endl;

        // With 4 threads processing 10 requests of 1ms each:
        // Should take approximately ceil(10/4) * 1ms = 3ms
        EXPECT_GE(duration.count(), 2);  // At least 2ms
        EXPECT_LE(duration.count(), 5);  // At most 5ms (with overhead)
    }
}

// Benchmark: Large batch of requests
TEST(RequestProcessingBenchmark, SequentialVsParallel_100Requests) {
    const int NUM_REQUESTS = 100;
    const auto REQUEST_DURATION = microseconds(500);  // 0.5ms per request
    const int NUM_THREADS = 4;

    // Sequential
    {
        std::atomic<int> counter{0};
        HandlersQueue queue;

        for (int i = 0; i < NUM_REQUESTS; ++i) {
            queue.Push(std::make_unique<SimulatedReadHandler>(REQUEST_DURATION, &counter));
        }

        auto start = high_resolution_clock::now();
        while (auto handler = queue.Pop()) {
            handler->Handle();
        }
        auto end = high_resolution_clock::now();
        auto duration = duration_cast<milliseconds>(end - start);

        EXPECT_EQ(NUM_REQUESTS, counter.load());
        std::cout << "Sequential (" << NUM_REQUESTS << " requests): "
                  << duration.count() << " ms" << std::endl;
    }

    // Parallel
    {
        std::atomic<int> counter{0};
        HandlersQueue queue;
        ThreadPool pool(NUM_THREADS);

        for (int i = 0; i < NUM_REQUESTS; ++i) {
            queue.Push(std::make_unique<SimulatedReadHandler>(REQUEST_DURATION, &counter));
        }

        auto start = high_resolution_clock::now();
        std::vector<std::function<void()>> tasks;
        while (auto handler = queue.Pop()) {
            auto h = std::shared_ptr<Handler>(std::move(handler));
            tasks.push_back([h]() { h->Handle(); });
        }
        pool.ExecuteAll(tasks);
        auto end = high_resolution_clock::now();
        auto duration = duration_cast<milliseconds>(end - start);

        EXPECT_EQ(NUM_REQUESTS, counter.load());
        std::cout << "Parallel   (" << NUM_REQUESTS << " requests, " << NUM_THREADS
                  << " threads): " << duration.count() << " ms" << std::endl;

        // Speedup calculation
        double sequential_time = 100 * 0.5;  // 50ms
        double parallel_time = duration.count();
        double speedup = sequential_time / parallel_time;

        std::cout << "Estimated speedup: " << speedup << "x" << std::endl;

        // Should see at least 2x speedup with 4 threads
        EXPECT_GT(speedup, 2.0);
    }
}

// Benchmark: Single request (overhead measurement)
TEST(RequestProcessingBenchmark, SingleRequest_Overhead) {
    const auto REQUEST_DURATION = microseconds(100);  // 0.1ms
    const int NUM_THREADS = 4;

    // Sequential (baseline)
    microseconds sequential_duration;
    {
        HandlersQueue queue;
        queue.Push(std::make_unique<SimulatedReadHandler>(REQUEST_DURATION));

        auto start = high_resolution_clock::now();
        auto handler = queue.Pop();
        handler->Handle();
        auto end = high_resolution_clock::now();
        sequential_duration = duration_cast<microseconds>(end - start);

        std::cout << "Sequential (1 request): " << sequential_duration.count() << " μs" << std::endl;
    }

    // Parallel (measure overhead)
    microseconds parallel_duration;
    {
        HandlersQueue queue;
        ThreadPool pool(NUM_THREADS);
        queue.Push(std::make_unique<SimulatedReadHandler>(REQUEST_DURATION));

        auto start = high_resolution_clock::now();
        std::vector<std::function<void()>> tasks;
        auto handler = queue.Pop();
        auto h = std::shared_ptr<Handler>(std::move(handler));
        tasks.push_back([h]() { h->Handle(); });
        pool.ExecuteAll(tasks);
        auto end = high_resolution_clock::now();
        parallel_duration = duration_cast<microseconds>(end - start);

        std::cout << "Parallel   (1 request): " << parallel_duration.count() << " μs" << std::endl;
    }

    // Calculate overhead
    auto overhead = parallel_duration - sequential_duration;
    std::cout << "Thread pool overhead: " << overhead.count() << " μs" << std::endl;

    // Overhead should be minimal (< 100μs)
    EXPECT_LT(overhead.count(), 100);
}

// Benchmark: Varying request counts
TEST(RequestProcessingBenchmark, VaryingRequestCounts) {
    const auto REQUEST_DURATION = microseconds(500);
    const int NUM_THREADS = 4;

    std::vector<int> request_counts = {1, 5, 10, 20, 50};

    std::cout << "\nRequest Count | Sequential (ms) | Parallel (ms) | Speedup\n";
    std::cout << "------------- | --------------- | ------------- | -------\n";

    for (int num_requests : request_counts) {
        // Sequential
        HandlersQueue seq_queue;
        for (int i = 0; i < num_requests; ++i) {
            seq_queue.Push(std::make_unique<SimulatedReadHandler>(REQUEST_DURATION));
        }

        auto seq_start = high_resolution_clock::now();
        while (auto handler = seq_queue.Pop()) {
            handler->Handle();
        }
        auto seq_end = high_resolution_clock::now();
        auto seq_duration = duration_cast<microseconds>(seq_end - seq_start);

        // Parallel
        HandlersQueue par_queue;
        ThreadPool pool(NUM_THREADS);
        for (int i = 0; i < num_requests; ++i) {
            par_queue.Push(std::make_unique<SimulatedReadHandler>(REQUEST_DURATION));
        }

        auto par_start = high_resolution_clock::now();
        std::vector<std::function<void()>> tasks;
        while (auto handler = par_queue.Pop()) {
            auto h = std::shared_ptr<Handler>(std::move(handler));
            tasks.push_back([h]() { h->Handle(); });
        }
        pool.ExecuteAll(tasks);
        auto par_end = high_resolution_clock::now();
        auto par_duration = duration_cast<microseconds>(par_end - par_start);

        double speedup = static_cast<double>(seq_duration.count()) / par_duration.count();

        printf("%13d | %15.2f | %13.2f | %6.2fx\n",
               num_requests,
               seq_duration.count() / 1000.0,
               par_duration.count() / 1000.0,
               speedup);

        // For request counts >= 4 (number of threads), expect speedup
        if (num_requests >= NUM_THREADS) {
            EXPECT_GT(speedup, 1.5);
        }
    }
}

// Benchmark: Mixed duration requests (realistic scenario)
TEST(RequestProcessingBenchmark, MixedDurationRequests) {
    const int NUM_THREADS = 4;

    // Simulate realistic scenario: mix of fast and slow requests
    std::vector<microseconds> request_durations = {
        microseconds(100),   // Fast: 0.1ms
        microseconds(500),   // Medium: 0.5ms
        microseconds(1000),  // Slow: 1ms
        microseconds(100),
        microseconds(2000),  // Very slow: 2ms
        microseconds(100),
        microseconds(500),
        microseconds(100),
        microseconds(1000),
        microseconds(500),
    };

    // Sequential
    microseconds seq_duration;
    {
        HandlersQueue queue;
        for (auto duration : request_durations) {
            queue.Push(std::make_unique<SimulatedReadHandler>(duration));
        }

        auto start = high_resolution_clock::now();
        while (auto handler = queue.Pop()) {
            handler->Handle();
        }
        auto end = high_resolution_clock::now();
        seq_duration = duration_cast<microseconds>(end - start);

        std::cout << "Sequential (mixed durations): " << seq_duration.count() / 1000.0 << " ms" << std::endl;
    }

    // Parallel
    microseconds par_duration;
    {
        HandlersQueue queue;
        ThreadPool pool(NUM_THREADS);
        for (auto duration : request_durations) {
            queue.Push(std::make_unique<SimulatedReadHandler>(duration));
        }

        auto start = high_resolution_clock::now();
        std::vector<std::function<void()>> tasks;
        while (auto handler = queue.Pop()) {
            auto h = std::shared_ptr<Handler>(std::move(handler));
            tasks.push_back([h]() { h->Handle(); });
        }
        pool.ExecuteAll(tasks);
        auto end = high_resolution_clock::now();
        par_duration = duration_cast<microseconds>(end - start);

        std::cout << "Parallel   (mixed durations): " << par_duration.count() / 1000.0 << " ms" << std::endl;
    }

    double speedup = static_cast<double>(seq_duration.count()) / par_duration.count();
    std::cout << "Speedup: " << speedup << "x" << std::endl;

    // Should see significant speedup even with mixed durations
    EXPECT_GT(speedup, 1.5);
}

// Benchmark: Verify correctness under load
TEST(RequestProcessingBenchmark, CorrectnessUnderLoad) {
    const int NUM_REQUESTS = 1000;
    const auto REQUEST_DURATION = microseconds(10);  // Very fast requests
    const int NUM_THREADS = 4;

    std::atomic<int> counter{0};
    HandlersQueue queue;
    ThreadPool pool(NUM_THREADS);

    for (int i = 0; i < NUM_REQUESTS; ++i) {
        queue.Push(std::make_unique<SimulatedReadHandler>(REQUEST_DURATION, &counter));
    }

    std::vector<std::function<void()>> tasks;
    while (auto handler = queue.Pop()) {
        auto h = std::shared_ptr<Handler>(std::move(handler));
        tasks.push_back([h]() { h->Handle(); });
    }

    auto start = high_resolution_clock::now();
    pool.ExecuteAll(tasks);
    auto end = high_resolution_clock::now();
    auto duration = duration_cast<milliseconds>(end - start);

    // Verify all requests processed
    EXPECT_EQ(NUM_REQUESTS, counter.load());

    std::cout << "Processed " << NUM_REQUESTS << " requests in "
              << duration.count() << " ms" << std::endl;
    std::cout << "Throughput: " << (NUM_REQUESTS * 1000.0) / duration.count()
              << " requests/second" << std::endl;

    // Should process quickly with parallel execution
    EXPECT_LT(duration.count(), 100);  // Should complete in < 100ms
}
