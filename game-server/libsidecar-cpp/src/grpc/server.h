#ifndef TC9_GRPC_SERVER_H
#define TC9_GRPC_SERVER_H

#include <memory>
#include <string>
#include <grpcpp/grpcpp.h>

namespace tc9 {

class GrpcServer {
public:
    explicit GrpcServer(const std::string& port);
    ~GrpcServer();

    void Start();
    void Shutdown();

    // Delete copy/move
    GrpcServer(const GrpcServer&) = delete;
    GrpcServer& operator=(const GrpcServer&) = delete;
    GrpcServer(GrpcServer&&) = delete;
    GrpcServer& operator=(GrpcServer&&) = delete;

private:
    std::unique_ptr<grpc::Server> server_;
    std::string port_;
};

}  // namespace tc9

#endif  // TC9_GRPC_SERVER_H
