#include "thread_pool.h"
#include <spdlog/spdlog.h>

namespace tc9 {

ThreadPool::ThreadPool(size_t num_threads) : stop_(false) {
    spdlog::info("Creating thread pool with {} threads", num_threads);

    for (size_t i = 0; i < num_threads; ++i) {
        workers_.emplace_back([this] { WorkerThread(); });
    }
}

ThreadPool::~ThreadPool() {
    Shutdown();
}

void ThreadPool::Submit(std::function<void()> task) {
    {
        std::lock_guard<std::mutex> lock(queue_mutex_);
        if (stop_) {
            spdlog::warn("Submitting task to stopped thread pool");
            return;
        }
        tasks_.push(std::move(task));
    }
    cv_.notify_one();
}

void ThreadPool::ExecuteAll(const std::vector<std::function<void()>>& tasks) {
    if (tasks.empty()) {
        return;
    }

    // Submit all tasks
    std::vector<std::future<void>> futures;
    futures.reserve(tasks.size());

    for (const auto& task : tasks) {
        auto promise = std::make_shared<std::promise<void>>();
        futures.push_back(promise->get_future());

        Submit([task, promise]() {
            try {
                task();
                promise->set_value();
            } catch (...) {
                promise->set_exception(std::current_exception());
            }
        });
    }

    // Wait for all tasks to complete
    for (auto& future : futures) {
        try {
            future.get();
        } catch (const std::exception& e) {
            spdlog::error("Task execution failed: {}", e.what());
        }
    }
}

void ThreadPool::Shutdown() {
    {
        std::lock_guard<std::mutex> lock(queue_mutex_);
        if (stop_) {
            return;  // Already stopped
        }
        stop_ = true;
    }

    cv_.notify_all();

    for (auto& worker : workers_) {
        if (worker.joinable()) {
            worker.join();
        }
    }

    spdlog::info("Thread pool shut down");
}

void ThreadPool::WorkerThread() {
    while (true) {
        std::function<void()> task;

        {
            std::unique_lock<std::mutex> lock(queue_mutex_);
            cv_.wait(lock, [this] { return stop_ || !tasks_.empty(); });

            if (stop_ && tasks_.empty()) {
                return;
            }

            if (!tasks_.empty()) {
                task = std::move(tasks_.front());
                tasks_.pop();
            }
        }

        if (task) {
            try {
                task();
            } catch (const std::exception& e) {
                spdlog::error("Worker thread caught exception: {}", e.what());
            } catch (...) {
                spdlog::error("Worker thread caught unknown exception");
            }
        }
    }
}

}  // namespace tc9
