# Multi-stage Dockerfile for libsidecar-cpp
# Builds a shared library compatible with AzerothCore
#
# Build from repo root:
#   docker build -f game-server/libsidecar-cpp/Dockerfile --target export -o out .
#
# Version: project(libsidecar VERSION ...) in CMakeLists.txt → generated tc9_version.h

FROM ubuntu:22.04 AS build-libsidecar

ENV DEBIAN_FRONTEND=noninteractive

WORKDIR /build

# Install build dependencies
RUN apt-get update && \
    apt-get install -y \
        git \
        cmake \
        make \
        gcc \
        g++ \
        build-essential \
        libssl-dev \
        zlib1g-dev \
        ca-certificates \
        pkg-config && \
    rm -rf /var/lib/apt/lists/*

# Copy source files
COPY api/proto /api/proto
COPY game-server/libsidecar-cpp /build/libsidecar-cpp

# Build libsidecar-cpp (clean tree — never reuse a host CMake cache)
WORKDIR /build/libsidecar-cpp
RUN rm -rf build && mkdir build && cd build && \
    cmake .. \
        -DCMAKE_BUILD_TYPE=Release \
        -DBUILD_TESTS=OFF \
        -DUSE_SYSTEM_GRPC=OFF \
        -DUSE_SYSTEM_PROTOBUF=OFF && \
    cmake --build . --config Release -j $(nproc)

# Extract artifacts: unversioned .so + public headers including generated version
RUN mkdir -p /artifacts/lib /artifacts/include && \
    cp build/libsidecar.so /artifacts/lib/ && \
    cp include/*.h /artifacts/include/ && \
    cp build/generated/include/tc9_version.h /artifacts/include/ && \
    cp -r include/libsidecar /artifacts/include/

# Minimal export stage
FROM scratch AS export
COPY --from=build-libsidecar /artifacts /
