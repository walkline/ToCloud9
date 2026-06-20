#ifndef TC9_GRPC_MANAGER_H
#define TC9_GRPC_MANAGER_H

#include "worldserver_service.h"
#include "../queue/handlers_queue.h"
#include <memory>
#include <string>
#include <grpcpp/grpcpp.h>

namespace tc9 {

class GrpcManager {
public:
    GrpcManager(
        const std::string& port,
        const CppBindings& bindings,
        HandlersQueue& read_queue,
        HandlersQueue& write_queue
    );
    ~GrpcManager();

    void Start();
    void Shutdown();

    GrpcManager(const GrpcManager&) = delete;
    GrpcManager& operator=(const GrpcManager&) = delete;

private:
    std::string port_;
    std::unique_ptr<WorldServerServiceImpl> service_;
    std::unique_ptr<grpc::Server> server_;
};

}  // namespace tc9

#endif  // TC9_GRPC_MANAGER_H
