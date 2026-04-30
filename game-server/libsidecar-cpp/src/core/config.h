#ifndef TC9_CONFIG_H
#define TC9_CONFIG_H

#include <string>
#include <cstdlib>

namespace tc9 {

class Config {
public:
    static Config& Instance();

    // Service addresses
    std::string grpc_port() const { return grpc_port_; }
    std::string health_check_port() const { return health_check_port_; }
    std::string preferred_hostname() const { return preferred_hostname_; }
    std::string servers_registry_address() const { return servers_registry_address_; }
    std::string matchmaking_address() const { return matchmaking_address_; }
    std::string guid_provider_address() const { return guid_provider_address_; }
    std::string nats_url() const { return nats_url_; }

    // Buffer sizes
    int character_guids_buffer_size() const { return character_guids_buffer_size_; }
    int item_guids_buffer_size() const { return item_guids_buffer_size_; }
    int instance_guids_buffer_size() const { return instance_guids_buffer_size_; }

    // Performance tuning
    int read_threads() const { return read_threads_; }
    bool parallel_read_processing() const { return parallel_read_processing_; }

    // Logging
    std::string log_level() const { return log_level_; }

    // Delete copy/move
    Config(const Config&) = delete;
    Config& operator=(const Config&) = delete;
    Config(Config&&) = delete;
    Config& operator=(Config&&) = delete;

private:
    Config();
    ~Config() = default;

    std::string GetEnv(const char* name, const std::string& default_value);
    int GetEnvInt(const char* name, int default_value);

    std::string grpc_port_;
    std::string health_check_port_;
    std::string preferred_hostname_;
    std::string servers_registry_address_;
    std::string matchmaking_address_;
    std::string guid_provider_address_;
    std::string nats_url_;
    int character_guids_buffer_size_;
    int item_guids_buffer_size_;
    int instance_guids_buffer_size_;
    int read_threads_;
    bool parallel_read_processing_;
    std::string log_level_;
};

}  // namespace tc9

#endif  // TC9_CONFIG_H
