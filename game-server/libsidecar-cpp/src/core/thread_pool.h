#ifndef TC9_THREAD_POOL_H
#define TC9_THREAD_POOL_H

#include <vector>
#include <queue>
#include <thread>
#include <mutex>
#include <condition_variable>
#include <functional>
#include <atomic>
#include <future>

namespace tc9 {

class ThreadPool {
public:
    explicit ThreadPool(size_t num_threads);
    ~ThreadPool();

    // Submit a task to the pool
    void Submit(std::function<void()> task);

    // Execute all tasks and wait for completion
    void ExecuteAll(const std::vector<std::function<void()>>& tasks);

    // Stop accepting new tasks and join all threads
    void Shutdown();

    // Delete copy/move
    ThreadPool(const ThreadPool&) = delete;
    ThreadPool& operator=(const ThreadPool&) = delete;
    ThreadPool(ThreadPool&&) = delete;
    ThreadPool& operator=(ThreadPool&&) = delete;

private:
    void WorkerThread();

    std::vector<std::thread> workers_;
    std::queue<std::function<void()>> tasks_;
    std::mutex queue_mutex_;
    std::condition_variable cv_;
    std::atomic<bool> stop_;
};

}  // namespace tc9

#endif  // TC9_THREAD_POOL_H
