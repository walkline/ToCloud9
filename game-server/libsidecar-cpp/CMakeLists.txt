cmake_minimum_required(VERSION 3.20)
project(libsidecar VERSION 0.0.1 LANGUAGES C CXX)

# C++ Standard
set(CMAKE_CXX_STANDARD 17)
set(CMAKE_CXX_STANDARD_REQUIRED ON)
set(CMAKE_CXX_EXTENSIONS OFF)

# Build type
if(NOT CMAKE_BUILD_TYPE)
    set(CMAKE_BUILD_TYPE Release)
endif()

# Options
option(BUILD_TESTS "Build unit tests" ON)
option(USE_SYSTEM_GRPC "Use system-installed gRPC" OFF)
option(USE_SYSTEM_PROTOBUF "Use system-installed Protobuf" OFF)
option(ENABLE_ASAN "Enable AddressSanitizer" OFF)
option(ENABLE_TSAN "Enable ThreadSanitizer" OFF)

# Platform detection
if(WIN32)
    add_definitions(-D_WIN32_WINNT=0x0601)  # Windows 7+
    add_definitions(-DNOMINMAX)
endif()

# Compiler flags
if(CMAKE_CXX_COMPILER_ID MATCHES "GNU|Clang")
    add_compile_options(
        -Wall
        -Wextra
        -Wpedantic
        -Wno-unused-parameter
    )

    if(CMAKE_BUILD_TYPE STREQUAL "Release")
        add_compile_options(-O3)
    endif()

    if(ENABLE_ASAN)
        add_compile_options(-fsanitize=address -fno-omit-frame-pointer)
        add_link_options(-fsanitize=address)
    endif()

    if(ENABLE_TSAN)
        add_compile_options(-fsanitize=thread -fno-omit-frame-pointer)
        add_link_options(-fsanitize=thread)
    endif()
elseif(MSVC)
    add_compile_options(
        /W4
        /wd4100  # Unused parameter
        /wd4251  # DLL interface
    )

    if(CMAKE_BUILD_TYPE STREQUAL "Release")
        add_compile_options(/O2)
    endif()

    # Export symbols for DLL
    add_definitions(-DTC9_EXPORTS)
endif()

# Dependencies
include(FetchContent)
set(FETCHCONTENT_QUIET OFF)

# Find threads (required by multiple dependencies)
find_package(Threads REQUIRED)

# gRPC and Protobuf
if(USE_SYSTEM_GRPC)
    find_package(gRPC CONFIG REQUIRED)
    find_package(Protobuf REQUIRED)
else()
    message(STATUS "Downloading and building gRPC (this may take 20-30 minutes)...")

    FetchContent_Declare(
        grpc
        GIT_REPOSITORY https://github.com/grpc/grpc.git
        GIT_TAG        v1.80.0
        GIT_SHALLOW    TRUE
    )
    set(gRPC_BUILD_TESTS OFF CACHE BOOL "" FORCE)
    set(gRPC_BUILD_CSHARP_EXT OFF CACHE BOOL "" FORCE)
    set(gRPC_BUILD_GRPC_CSHARP_PLUGIN OFF CACHE BOOL "" FORCE)
    set(gRPC_BUILD_GRPC_NODE_PLUGIN OFF CACHE BOOL "" FORCE)
    set(gRPC_BUILD_GRPC_OBJECTIVE_C_PLUGIN OFF CACHE BOOL "" FORCE)
    set(gRPC_BUILD_GRPC_PHP_PLUGIN OFF CACHE BOOL "" FORCE)
    set(gRPC_BUILD_GRPC_PYTHON_PLUGIN OFF CACHE BOOL "" FORCE)
    set(gRPC_BUILD_GRPC_RUBY_PLUGIN OFF CACHE BOOL "" FORCE)
    set(gRPC_INSTALL OFF CACHE BOOL "" FORCE)
    set(protobuf_INSTALL OFF CACHE BOOL "" FORCE)
    set(utf8_range_ENABLE_INSTALL OFF CACHE BOOL "" FORCE)
    set(gRPC_ZLIB_PROVIDER "package" CACHE STRING "" FORCE)
    FetchContent_MakeAvailable(grpc)

    # Set variables for consistency
    set(_PROTOBUF_LIBPROTOBUF libprotobuf)
    set(_REFLECTION grpc++_reflection)
    set(_GRPC_GRPCPP grpc++)
    set(_GRPC_CPP_PLUGIN_EXECUTABLE $<TARGET_FILE:grpc_cpp_plugin>)
    set(_PROTOBUF_PROTOC $<TARGET_FILE:protoc>)
endif()

# spdlog
FetchContent_Declare(
    spdlog
    GIT_REPOSITORY https://github.com/gabime/spdlog.git
    GIT_TAG        v1.13.0
    GIT_SHALLOW    TRUE
)
FetchContent_MakeAvailable(spdlog)

# prometheus-cpp
FetchContent_Declare(
    prometheus-cpp
    GIT_REPOSITORY https://github.com/jupp0r/prometheus-cpp.git
    GIT_TAG        v1.2.4
    GIT_SHALLOW    TRUE
)
set(ENABLE_PUSH OFF CACHE BOOL "" FORCE)
set(ENABLE_COMPRESSION OFF CACHE BOOL "" FORCE)
set(ENABLE_TESTING OFF CACHE BOOL "" FORCE)
FetchContent_MakeAvailable(prometheus-cpp)

# cpp-httplib
set(HTTPLIB_REQUIRE_OPENSSL OFF CACHE BOOL "" FORCE)
set(HTTPLIB_USE_OPENSSL_IF_AVAILABLE OFF CACHE BOOL "" FORCE)
FetchContent_Declare(
    httplib
    GIT_REPOSITORY https://github.com/yhirose/cpp-httplib.git
    GIT_TAG        v0.43.1
    GIT_SHALLOW    TRUE
)
FetchContent_MakeAvailable(httplib)

# nats.c
FetchContent_Declare(
    nats
    GIT_REPOSITORY https://github.com/nats-io/nats.c.git
    GIT_TAG        v3.8.2
    GIT_SHALLOW    TRUE
)
set(NATS_BUILD_STREAMING OFF CACHE BOOL "" FORCE)
set(NATS_BUILD_EXAMPLES OFF CACHE BOOL "" FORCE)
set(NATS_BUILD_TESTING OFF CACHE BOOL "" FORCE)
FetchContent_MakeAvailable(nats)

# nlohmann/json for JSON parsing
FetchContent_Declare(
    json
    GIT_REPOSITORY https://github.com/nlohmann/json.git
    GIT_TAG        v3.11.3
    GIT_SHALLOW    TRUE
)
set(JSON_BuildTests OFF CACHE BOOL "" FORCE)
set(JSON_Install OFF CACHE BOOL "" FORCE)
FetchContent_MakeAvailable(json)

# Protobuf generation
set(PROTO_PATH "${CMAKE_SOURCE_DIR}/../../api/proto/v1")
set(PROTO_FILES
    "worldserver/worldserver.proto"
    "servers-registry/registry.proto"
    "matchmaking/matchmaking.proto"
    "guid/guid.proto"
)

set(PROTO_GENERATED_DIR "${CMAKE_BINARY_DIR}/generated")
file(MAKE_DIRECTORY ${PROTO_GENERATED_DIR})

set(PROTO_SRCS)
set(PROTO_HDRS)
set(GRPC_SRCS)
set(GRPC_HDRS)

foreach(PROTO_FILE ${PROTO_FILES})
    get_filename_component(PROTO_NAME ${PROTO_FILE} NAME_WE)
    get_filename_component(PROTO_SUBDIR ${PROTO_FILE} DIRECTORY)

    # Protoc preserves directory structure in output
    set(PROTO_SRC "${PROTO_GENERATED_DIR}/${PROTO_SUBDIR}/${PROTO_NAME}.pb.cc")
    set(PROTO_HDR "${PROTO_GENERATED_DIR}/${PROTO_SUBDIR}/${PROTO_NAME}.pb.h")
    set(GRPC_SRC "${PROTO_GENERATED_DIR}/${PROTO_SUBDIR}/${PROTO_NAME}.grpc.pb.cc")
    set(GRPC_HDR "${PROTO_GENERATED_DIR}/${PROTO_SUBDIR}/${PROTO_NAME}.grpc.pb.h")

    list(APPEND PROTO_SRCS ${PROTO_SRC})
    list(APPEND PROTO_HDRS ${PROTO_HDR})
    list(APPEND GRPC_SRCS ${GRPC_SRC})
    list(APPEND GRPC_HDRS ${GRPC_HDR})

    add_custom_command(
        OUTPUT ${PROTO_SRC} ${PROTO_HDR} ${GRPC_SRC} ${GRPC_HDR}
        COMMAND ${_PROTOBUF_PROTOC}
        ARGS --grpc_out=${PROTO_GENERATED_DIR}
             --cpp_out=${PROTO_GENERATED_DIR}
             -I${PROTO_PATH}
             --plugin=protoc-gen-grpc=${_GRPC_CPP_PLUGIN_EXECUTABLE}
             ${PROTO_PATH}/${PROTO_FILE}
        DEPENDS ${PROTO_PATH}/${PROTO_FILE}
        COMMENT "Generating C++ code for ${PROTO_FILE}"
    )
endforeach()

# Create protobuf library target
add_library(libsidecar_proto STATIC
    ${PROTO_SRCS}
    ${PROTO_HDRS}
    ${GRPC_SRCS}
    ${GRPC_HDRS}
)

target_include_directories(libsidecar_proto PUBLIC
    ${PROTO_GENERATED_DIR}
)

target_link_libraries(libsidecar_proto PRIVATE
    ${_PROTOBUF_LIBPROTOBUF}
    ${_REFLECTION}
    ${_GRPC_GRPCPP}
)

# Main library sources
set(LIBSIDECAR_SOURCES
    src/core/config.cpp
    src/core/logger.cpp
    src/core/error.cpp
    src/core/thread_pool.cpp
    src/queue/handlers_queue.cpp
    src/grpc/server.cpp
    src/grpc/clients.cpp
    src/grpc/handlers.cpp
    src/grpc/worldserver_service.cpp
    src/grpc/grpc_manager.cpp
    src/nats/consumer.cpp
    src/nats/handlers.cpp
    src/http/health_server.cpp
    src/metrics/prometheus.cpp
    src/guids/guid_manager.cpp
    src/events/event_hooks.cpp
    src/compat_api.cpp
    src/api.cpp
)

# Main library target
add_library(libsidecar SHARED ${LIBSIDECAR_SOURCES})

# Library naming to match Go artifacts (libsidecar.so on all platforms)
set_target_properties(libsidecar PROPERTIES
    PREFIX ""
    OUTPUT_NAME "libsidecar"
    VERSION ${PROJECT_VERSION}
    SOVERSION ${PROJECT_VERSION_MAJOR}
)

target_include_directories(libsidecar
    PUBLIC
        $<BUILD_INTERFACE:${CMAKE_SOURCE_DIR}/include>
        $<INSTALL_INTERFACE:include>
    PRIVATE
        ${CMAKE_SOURCE_DIR}/src
        ${PROTO_GENERATED_DIR}
)

# Disable HTTPS support in cpp-httplib (we only need HTTP)
target_compile_definitions(libsidecar PRIVATE
    CPPHTTPLIB_OPENSSL_SUPPORT=0
)

target_link_libraries(libsidecar
    PRIVATE
        libsidecar_proto
        ${_GRPC_GRPCPP}
        ${_PROTOBUF_LIBPROTOBUF}
        spdlog::spdlog
        prometheus-cpp::core
        prometheus-cpp::pull
        httplib::httplib
        nats
        nlohmann_json::nlohmann_json
        Threads::Threads
)

# Platform-specific libraries
if(UNIX AND NOT APPLE)
    target_link_libraries(libsidecar PRIVATE ${CMAKE_DL_LIBS})
elseif(WIN32)
    target_link_libraries(libsidecar PRIVATE ws2_32 wsock32)
endif()

# Export symbols on Windows
if(WIN32)
    target_compile_definitions(libsidecar PRIVATE TC9_BUILDING_DLL)
endif()

# Tests
if(BUILD_TESTS)
    enable_testing()

    FetchContent_Declare(
        googletest
        GIT_REPOSITORY https://github.com/google/googletest.git
        GIT_TAG        v1.14.0
        GIT_SHALLOW    TRUE
    )
    set(gtest_force_shared_crt ON CACHE BOOL "" FORCE)
    FetchContent_MakeAvailable(googletest)

    add_executable(libsidecar_tests
        tests/queue_test.cpp
        tests/thread_pool_test.cpp
        tests/config_test.cpp
        tests/error_test.cpp
        tests/guid_manager_test.cpp
        tests/request_processing_benchmark.cpp
        tests/request_processing_realistic_benchmark.cpp
    )

    target_link_libraries(libsidecar_tests
        PRIVATE
            libsidecar
            gtest_main
    )

    include(GoogleTest)
    gtest_discover_tests(libsidecar_tests)

    # C API compatibility test (pure C code)
    add_executable(c_api_compatibility_test
        tests/c_api_compatibility_test.c
    )

    set_target_properties(c_api_compatibility_test PROPERTIES
        LINKER_LANGUAGE C
    )

    target_include_directories(c_api_compatibility_test PRIVATE
        ${CMAKE_SOURCE_DIR}/include
    )

    target_link_libraries(c_api_compatibility_test
        PRIVATE
            libsidecar
    )

    add_test(NAME CApiCompatibility COMMAND c_api_compatibility_test)
endif()

# Installation
include(GNUInstallDirs)

install(TARGETS libsidecar
    LIBRARY DESTINATION ${CMAKE_INSTALL_LIBDIR}
    ARCHIVE DESTINATION ${CMAKE_INSTALL_LIBDIR}
    RUNTIME DESTINATION ${CMAKE_INSTALL_BINDIR}
)

# Install compatibility headers to root include directory (matching Go structure)
install(FILES
    include/libsidecar.h
    include/events-guild.h
    include/events-group.h
    include/events-servers-registry.h
    include/player-items-api.h
    include/player-money-api.h
    include/player-interactions-api.h
    include/battleground-api.h
    include/monitoring.h
    DESTINATION ${CMAKE_INSTALL_INCLUDEDIR}
)

# Keep C++ internal headers in subdirectory
install(DIRECTORY include/libsidecar
    DESTINATION ${CMAKE_INSTALL_INCLUDEDIR}
)

# Print summary
message(STATUS "")
message(STATUS "=== libsidecar-cpp Configuration ===")
message(STATUS "Version: ${PROJECT_VERSION}")
message(STATUS "Build type: ${CMAKE_BUILD_TYPE}")
message(STATUS "C++ standard: ${CMAKE_CXX_STANDARD}")
message(STATUS "Build tests: ${BUILD_TESTS}")
message(STATUS "Use system gRPC: ${USE_SYSTEM_GRPC}")
message(STATUS "ASAN enabled: ${ENABLE_ASAN}")
message(STATUS "TSAN enabled: ${ENABLE_TSAN}")
message(STATUS "=====================================")
message(STATUS "")
