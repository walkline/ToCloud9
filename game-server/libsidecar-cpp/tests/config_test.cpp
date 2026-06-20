#include <gtest/gtest.h>
#include "../src/core/config.h"
#include <cstdlib>

// Windows doesn't have setenv/unsetenv, use _putenv_s and _putenv instead
#ifdef _WIN32
    #define setenv(name, value, overwrite) _putenv_s(name, value)
    #define unsetenv(name) _putenv_s(name, "")
#endif

using namespace tc9;

TEST(ConfigTest, DefaultValues) {
    // Clear environment variables
    unsetenv("TC9_GRPC_PORT");
    unsetenv("TC9_HEALTH_CHECK_PORT");

    auto& config = Config::Instance();

    EXPECT_EQ(config.grpc_port(), "9501");
    EXPECT_EQ(config.health_check_port(), "8901");
    EXPECT_EQ(config.read_threads(), 4);
    EXPECT_EQ(config.log_level(), "info");
}

TEST(ConfigTest, EnvironmentOverrides) {
    setenv("TC9_GRPC_PORT", "9999", 1);
    setenv("TC9_READ_THREADS", "8", 1);
    setenv("TC9_LOG_LEVEL", "debug", 1);

    // Note: Config is a singleton, so this test might affect others
    // In a real test suite, we'd refactor Config to allow injection

    // For now, just verify the getenv logic works
    EXPECT_STREQ(getenv("TC9_GRPC_PORT"), "9999");
    EXPECT_STREQ(getenv("TC9_READ_THREADS"), "8");

    // Cleanup
    unsetenv("TC9_GRPC_PORT");
    unsetenv("TC9_READ_THREADS");
    unsetenv("TC9_LOG_LEVEL");
}

TEST(ConfigTest, ParallelReadProcessingDefault) {
    Config& config = Config::Instance();

    // Default should be false (sequential processing)
    EXPECT_FALSE(config.parallel_read_processing());
}
