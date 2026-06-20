#include <gtest/gtest.h>
#include "../src/core/thread_pool.h"
#include "../src/queue/handlers_queue.h"
#include <chrono>
#include <vector>
#include <atomic>
#include <cstring>
#include <random>

using namespace tc9;
using namespace std::chrono;

// Simulate realistic read operation: data lookups and copying
class RealisticReadHandler : public Handler {
public:
    RealisticReadHandler(std::atomic<int>* counter, std::vector<uint64_t>* output)
        : counter_(counter), output_(output) {}

    void Handle() override {
        // Simulate realistic work that a read handler might do:
        // 1. Look up data in a hash map (simulated with array lookup)
        // 2. Copy some data structures
        // 3. Perform simple calculations

        static thread_local std::vector<uint64_t> item_guids = {
            1000, 2000, 3000, 4000, 5000, 6000, 7000, 8000, 9000, 10000
        };

        // Simulate item lookup and data copy (like GetPlayerItemsByGuids)
        std::vector<uint64_t> result;
        result.reserve(10);

        for (size_t i = 0; i < item_guids.size(); ++i) {
            // Simulate some simple processing
            uint64_t guid = item_guids[i];
            uint64_t processed = guid * 2 + i;  // Simple calculation
            result.push_back(processed);
        }

        // Store result (simulate response)
        if (output_) {
            *output_ = result;
        }

        if (counter_) {
            counter_->fetch_add(1);
        }
    }

private:
    std::atomic<int>* counter_;
    std::vector<uint64_t>* output_;
};

// Even more realistic: actual memory operations
class MemoryIntensiveHandler : public Handler {
public:
    MemoryIntensiveHandler(std::atomic<int>* counter) : counter_(counter) {}

    void Handle() override {
        // Simulate realistic memory-intensive operations:
        // - String operations (player names, etc.)
        // - Structure copies (item data, etc.)
        // - Vector operations (item lists, etc.)

        const int NUM_ITEMS = 50;

        // Simulate item data structure
        struct ItemData {
            uint64_t guid;
            uint32_t entry;
            uint32_t count;
            char name[64];
        };

        std::vector<ItemData> items;
        items.reserve(NUM_ITEMS);

        // Simulate populating item data
        for (int i = 0; i < NUM_ITEMS; ++i) {
            ItemData item;
            item.guid = 1000000 + i;
            item.entry = 25000 + (i % 100);
            item.count = 1 + (i % 20);
            snprintf(item.name, sizeof(item.name), "Item_%d", i);
            items.push_back(item);
        }

        // Simulate some processing (filtering, sorting, etc.)
        uint64_t checksum = 0;
        for (const auto& item : items) {
            checksum += item.guid + item.entry + item.count;
        }

        // Prevent optimization
        volatile uint64_t result = checksum;
        (void)result;

        if (counter_) {
            counter_->fetch_add(1);
        }
    }

private:
    std::atomic<int>* counter_;
};

// Benchmark: Empty handler (absolute minimum overhead)
TEST(RealisticBenchmark, EmptyHandler_Overhead) {
    const int NUM_REQUESTS = 1000;
    const int NUM_THREADS = 4;

    class EmptyHandler : public Handler {
    public:
        void Handle() override {}
    };

    // Sequential
    {
        HandlersQueue queue;
        for (int i = 0; i < NUM_REQUESTS; ++i) {
            queue.Push(std::make_unique<EmptyHandler>());
        }

        auto start = high_resolution_clock::now();
        while (auto handler = queue.Pop()) {
            handler->Handle();
        }
        auto end = high_resolution_clock::now();
        auto duration = duration_cast<microseconds>(end - start);

        std::cout << "Sequential (empty handlers): " << duration.count() << " μs "
                  << "(" << duration.count() / (double)NUM_REQUESTS << " μs/request)" << std::endl;
    }

    // Parallel
    {
        HandlersQueue queue;
        ThreadPool pool(NUM_THREADS);
        for (int i = 0; i < NUM_REQUESTS; ++i) {
            queue.Push(std::make_unique<EmptyHandler>());
        }

        auto start = high_resolution_clock::now();
        std::vector<std::function<void()>> tasks;
        while (auto handler = queue.Pop()) {
            auto h = std::shared_ptr<Handler>(std::move(handler));
            tasks.push_back([h]() { h->Handle(); });
        }
        pool.ExecuteAll(tasks);
        auto end = high_resolution_clock::now();
        auto duration = duration_cast<microseconds>(end - start);

        std::cout << "Parallel   (empty handlers): " << duration.count() << " μs "
                  << "(" << duration.count() / (double)NUM_REQUESTS << " μs/request)" << std::endl;
    }
}

// Benchmark: Realistic read operations
TEST(RealisticBenchmark, RealisticReadOperations) {
    const int NUM_REQUESTS = 100;
    const int NUM_THREADS = 4;

    // Sequential
    microseconds seq_duration;
    {
        std::atomic<int> counter{0};
        HandlersQueue queue;
        std::vector<std::vector<uint64_t>> outputs(NUM_REQUESTS);

        for (int i = 0; i < NUM_REQUESTS; ++i) {
            queue.Push(std::make_unique<RealisticReadHandler>(&counter, &outputs[i]));
        }

        auto start = high_resolution_clock::now();
        while (auto handler = queue.Pop()) {
            handler->Handle();
        }
        auto end = high_resolution_clock::now();
        seq_duration = duration_cast<microseconds>(end - start);

        EXPECT_EQ(NUM_REQUESTS, counter.load());
        std::cout << "Sequential (realistic reads): " << seq_duration.count() << " μs "
                  << "(" << seq_duration.count() / (double)NUM_REQUESTS << " μs/request)" << std::endl;
    }

    // Parallel
    microseconds par_duration;
    {
        std::atomic<int> counter{0};
        HandlersQueue queue;
        ThreadPool pool(NUM_THREADS);
        std::vector<std::vector<uint64_t>> outputs(NUM_REQUESTS);

        for (int i = 0; i < NUM_REQUESTS; ++i) {
            queue.Push(std::make_unique<RealisticReadHandler>(&counter, &outputs[i]));
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

        EXPECT_EQ(NUM_REQUESTS, counter.load());
        std::cout << "Parallel   (realistic reads): " << par_duration.count() << " μs "
                  << "(" << par_duration.count() / (double)NUM_REQUESTS << " μs/request)" << std::endl;
    }

    double speedup = static_cast<double>(seq_duration.count()) / par_duration.count();
    std::cout << "Speedup: " << speedup << "x\n" << std::endl;
}

// Benchmark: Memory-intensive operations
TEST(RealisticBenchmark, MemoryIntensiveOperations) {
    const int NUM_REQUESTS = 100;
    const int NUM_THREADS = 4;

    // Sequential
    microseconds seq_duration;
    {
        std::atomic<int> counter{0};
        HandlersQueue queue;

        for (int i = 0; i < NUM_REQUESTS; ++i) {
            queue.Push(std::make_unique<MemoryIntensiveHandler>(&counter));
        }

        auto start = high_resolution_clock::now();
        while (auto handler = queue.Pop()) {
            handler->Handle();
        }
        auto end = high_resolution_clock::now();
        seq_duration = duration_cast<microseconds>(end - start);

        EXPECT_EQ(NUM_REQUESTS, counter.load());
        std::cout << "Sequential (memory-intensive): " << seq_duration.count() << " μs "
                  << "(" << seq_duration.count() / (double)NUM_REQUESTS << " μs/request)" << std::endl;
    }

    // Parallel
    microseconds par_duration;
    {
        std::atomic<int> counter{0};
        HandlersQueue queue;
        ThreadPool pool(NUM_THREADS);

        for (int i = 0; i < NUM_REQUESTS; ++i) {
            queue.Push(std::make_unique<MemoryIntensiveHandler>(&counter));
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

        EXPECT_EQ(NUM_REQUESTS, counter.load());
        std::cout << "Parallel   (memory-intensive): " << par_duration.count() << " μs "
                  << "(" << par_duration.count() / (double)NUM_REQUESTS << " μs/request)" << std::endl;
    }

    double speedup = static_cast<double>(seq_duration.count()) / par_duration.count();
    std::cout << "Speedup: " << speedup << "x\n" << std::endl;
}

// Benchmark: Varying request counts with realistic work
TEST(RealisticBenchmark, ScalabilityWithRealisticWork) {
    const int NUM_THREADS = 4;
    std::vector<int> request_counts = {1, 5, 10, 20, 50, 100};

    std::cout << "\n=== Realistic Work Scalability ===\n";
    std::cout << "Requests | Sequential (μs) | Parallel (μs) | Speedup | Blocking Time\n";
    std::cout << "-------- | --------------- | ------------- | ------- | -------------\n";

    for (int num_requests : request_counts) {
        // Sequential
        std::atomic<int> seq_counter{0};
        HandlersQueue seq_queue;
        for (int i = 0; i < num_requests; ++i) {
            seq_queue.Push(std::make_unique<MemoryIntensiveHandler>(&seq_counter));
        }

        auto seq_start = high_resolution_clock::now();
        while (auto handler = seq_queue.Pop()) {
            handler->Handle();
        }
        auto seq_end = high_resolution_clock::now();
        auto seq_duration = duration_cast<microseconds>(seq_end - seq_start);

        // Parallel
        std::atomic<int> par_counter{0};
        HandlersQueue par_queue;
        ThreadPool pool(NUM_THREADS);
        for (int i = 0; i < num_requests; ++i) {
            par_queue.Push(std::make_unique<MemoryIntensiveHandler>(&par_counter));
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

        printf("%8d | %15ld | %13ld | %6.2fx | %6.2f ms\n",
               num_requests,
               seq_duration.count(),
               par_duration.count(),
               speedup,
               par_duration.count() / 1000.0);

        EXPECT_EQ(num_requests, seq_counter.load());
        EXPECT_EQ(num_requests, par_counter.load());
    }

    std::cout << "\nNote: 'Blocking Time' = time the game loop is blocked waiting for handlers\n" << std::endl;
}

// Benchmark: Worst case - very fast handlers
TEST(RealisticBenchmark, WorstCase_VeryFastHandlers) {
    const int NUM_REQUESTS = 1000;
    const int NUM_THREADS = 4;

    class TinyHandler : public Handler {
    public:
        explicit TinyHandler(std::atomic<uint64_t>* sum) : sum_(sum) {}
        void Handle() override {
            // Minimal work - just atomic increment
            sum_->fetch_add(1);
        }
    private:
        std::atomic<uint64_t>* sum_;
    };

    std::cout << "\n=== Worst Case: Ultra-Fast Handlers ===\n";

    // Sequential
    microseconds seq_duration;
    {
        std::atomic<uint64_t> sum{0};
        HandlersQueue queue;
        for (int i = 0; i < NUM_REQUESTS; ++i) {
            queue.Push(std::make_unique<TinyHandler>(&sum));
        }

        auto start = high_resolution_clock::now();
        while (auto handler = queue.Pop()) {
            handler->Handle();
        }
        auto end = high_resolution_clock::now();
        seq_duration = duration_cast<microseconds>(end - start);

        std::cout << "Sequential: " << seq_duration.count() << " μs "
                  << "(" << (seq_duration.count() / (double)NUM_REQUESTS) << " ns/request)" << std::endl;
        EXPECT_EQ(NUM_REQUESTS, sum.load());
    }

    // Parallel
    microseconds par_duration;
    {
        std::atomic<uint64_t> sum{0};
        HandlersQueue queue;
        ThreadPool pool(NUM_THREADS);
        for (int i = 0; i < NUM_REQUESTS; ++i) {
            queue.Push(std::make_unique<TinyHandler>(&sum));
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

        std::cout << "Parallel:   " << par_duration.count() << " μs "
                  << "(" << (par_duration.count() / (double)NUM_REQUESTS) << " ns/request)" << std::endl;
        EXPECT_EQ(NUM_REQUESTS, sum.load());
    }

    double speedup = static_cast<double>(seq_duration.count()) / par_duration.count();
    std::cout << "Speedup: " << speedup << "x\n";
    std::cout << "Note: For ultra-fast handlers, thread pool overhead may outweigh benefits\n" << std::endl;
}
