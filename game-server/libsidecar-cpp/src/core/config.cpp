#include "config.h"
#include <cstdlib>

namespace tc9 {

Config& Config::Instance() {
    static Config instance;
    return instance;
}

Config::Config() {
    grpc_port_ = GetEnv("TC9_GRPC_PORT", "9501");
    health_check_port_ = GetEnv("TC9_HEALTH_CHECK_PORT", "8901");
    preferred_hostname_ = GetEnv("TC9_PREFERRED_HOSTNAME", "");
    servers_registry_address_ = GetEnv("TC9_SERVERS_REGISTRY_ADDRESS", "localhost:8999");
    matchmaking_address_ = GetEnv("TC9_MATCHMAKING_ADDRESS", "localhost:8994");
    guid_provider_address_ = GetEnv("TC9_GUID_PROVIDER_ADDRESS", "localhost:8996");
    nats_url_ = GetEnv("TC9_NATS_URL", "nats://localhost:4222");

    character_guids_buffer_size_ = GetEnvInt("TC9_CHARACTER_GUIDS_BUFFER_SIZE", 50);
    item_guids_buffer_size_ = GetEnvInt("TC9_ITEM_GUIDS_BUFFER_SIZE", 200);
    instance_guids_buffer_size_ = GetEnvInt("TC9_INSTANCE_GUIDS_BUFFER_SIZE", 10);

    read_threads_ = GetEnvInt("TC9_READ_THREADS", 4);

    // Parallel read processing: off by default (sequential is faster for typical fast operations)
    // Set TC9_PARALLEL_READ_PROCESSING=1 to enable if you have slow read operations
    parallel_read_processing_ = GetEnvInt("TC9_PARALLEL_READ_PROCESSING", 0) != 0;

    log_level_ = GetEnv("TC9_LOG_LEVEL", "info");
}

std::string Config::GetEnv(const char* name, const std::string& default_value) {
    const char* value = std::getenv(name);
    return value ? std::string(value) : default_value;
}

int Config::GetEnvInt(const char* name, int default_value) {
    const char* value = std::getenv(name);
    if (!value) {
        return default_value;
    }

    try {
        return std::stoi(value);
    } catch (...) {
        return default_value;
    }
}

}  // namespace tc9
