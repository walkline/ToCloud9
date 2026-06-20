#include "server.h"
#include "worldserver_service.h"
#include <spdlog/spdlog.h>
#include <grpcpp/server_builder.h>

namespace tc9 {

GrpcServer::GrpcServer(const std::string& port) : port_(port) {
    spdlog::info("GrpcServer created for port {}", port);
}

GrpcServer::~GrpcServer() {
    Shutdown();
}

void GrpcServer::Start() {
    spdlog::info("Starting gRPC server on port {}", port_);
    // Server is started in the constructor via ServerBuilder
    // This method is kept for API compatibility
}

void GrpcServer::Shutdown() {
    if (server_) {
        spdlog::info("Shutting down gRPC server");
        server_->Shutdown();
        server_.reset();
    }
}

}  // namespace tc9
