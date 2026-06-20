#include "handlers_queue.h"

namespace tc9 {

void HandlersQueue::Push(std::unique_ptr<Handler> handler) {
    if (!handler) {
        return;
    }

    std::lock_guard<std::mutex> lock(mutex_);
    queue_.push(std::move(handler));
}

std::unique_ptr<Handler> HandlersQueue::Pop() {
    std::lock_guard<std::mutex> lock(mutex_);

    if (queue_.empty()) {
        return nullptr;
    }

    auto handler = std::move(queue_.front());
    queue_.pop();
    return handler;
}

bool HandlersQueue::Empty() const {
    std::lock_guard<std::mutex> lock(mutex_);
    return queue_.empty();
}

size_t HandlersQueue::Size() const {
    std::lock_guard<std::mutex> lock(mutex_);
    return queue_.size();
}

}  // namespace tc9
