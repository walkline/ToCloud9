#ifndef TC9_HANDLERS_QUEUE_H
#define TC9_HANDLERS_QUEUE_H

#include <queue>
#include <mutex>
#include <functional>
#include <memory>

namespace tc9 {

// Handler interface
class Handler {
public:
    virtual ~Handler() = default;
    virtual void Handle() = 0;
};

// Function handler implementation
class FunctionHandler : public Handler {
public:
    explicit FunctionHandler(std::function<void()> func)
        : func_(std::move(func)) {}

    void Handle() override {
        func_();
    }

private:
    std::function<void()> func_;
};

// Helper to create handlers
inline std::unique_ptr<Handler> MakeHandler(std::function<void()> func) {
    return std::make_unique<FunctionHandler>(std::move(func));
}

// Thread-safe FIFO queue
class HandlersQueue {
public:
    HandlersQueue() = default;
    ~HandlersQueue() = default;

    // Push a handler to the queue
    void Push(std::unique_ptr<Handler> handler);

    // Pop a handler from the queue (non-blocking, returns nullptr if empty)
    std::unique_ptr<Handler> Pop();

    // Check if queue is empty
    bool Empty() const;

    // Get queue size
    size_t Size() const;

    // Delete copy/move
    HandlersQueue(const HandlersQueue&) = delete;
    HandlersQueue& operator=(const HandlersQueue&) = delete;
    HandlersQueue(HandlersQueue&&) = delete;
    HandlersQueue& operator=(HandlersQueue&&) = delete;

private:
    std::queue<std::unique_ptr<Handler>> queue_;
    mutable std::mutex mutex_;
};

}  // namespace tc9

#endif  // TC9_HANDLERS_QUEUE_H
