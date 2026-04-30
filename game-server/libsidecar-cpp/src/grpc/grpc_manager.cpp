#include "grpc_manager.h"
#include <spdlog/spdlog.h>
#include <grpcpp/server_builder.h>

namespace tc9 {

GrpcManager::GrpcManager(
    const std::string& port,
    const CppBindings& bindings,
    HandlersQueue& read_queue,
    HandlersQueue& write_queue)
    : port_(port) {

    spdlog::info("Creating gRPC server on port {}", port);

    // Create service with 5 second timeout
    service_ = std::make_unique<WorldServerServiceImpl>(
        bindings,
        std::chrono::seconds(5),
        read_queue,
        write_queue
    );
}

GrpcManager::~GrpcManager() {
    Shutdown();
}

void GrpcManager::Start() {
    spdlog::info("Starting gRPC server on port {}", port_);

    std::string server_address = "0.0.0.0:" + port_;

    grpc::ServerBuilder builder;
    builder.AddListeningPort(server_address, grpc::InsecureServerCredentials());
    builder.RegisterService(service_.get());

    server_ = builder.BuildAndStart();

    if (!server_) {
        throw std::runtime_error("Failed to start gRPC server");
    }

    spdlog::info("gRPC server listening on {}", server_address);
}

void GrpcManager::Shutdown() {
    if (server_) {
        spdlog::info("Shutting down gRPC server");
        server_->Shutdown();
        server_.reset();
    }
}

}  // namespace tc9
