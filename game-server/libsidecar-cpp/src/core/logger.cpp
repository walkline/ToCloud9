#include "logger.h"
#include <spdlog/sinks/stdout_color_sinks.h>

namespace tc9 {

void InitLogger(const std::string& level) {
    auto console_sink = std::make_shared<spdlog::sinks::stdout_color_sink_mt>();
    auto logger = std::make_shared<spdlog::logger>("tc9", console_sink);

    // Set level
    if (level == "trace") {
        logger->set_level(spdlog::level::trace);
    } else if (level == "debug") {
        logger->set_level(spdlog::level::debug);
    } else if (level == "info") {
        logger->set_level(spdlog::level::info);
    } else if (level == "warn") {
        logger->set_level(spdlog::level::warn);
    } else if (level == "error") {
        logger->set_level(spdlog::level::err);
    } else {
        logger->set_level(spdlog::level::info);
    }

    // Set pattern: [timestamp] [level] message
    logger->set_pattern("[%Y-%m-%d %H:%M:%S.%e] [%^%l%$] %v");

    spdlog::set_default_logger(logger);
    spdlog::info("Logger initialized with level: {}", level);
}

void ShutdownLogger() {
    spdlog::info("Shutting down logger");
    spdlog::shutdown();
}

}  // namespace tc9
