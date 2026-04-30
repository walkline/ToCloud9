#include <gtest/gtest.h>
#include "../src/core/error.h"

using namespace tc9;

TEST(ErrorTest, ErrorCodeToString) {
    EXPECT_STREQ(ErrorCodeToString(TC9_ERROR_SUCCESS), "Success");
    EXPECT_STREQ(ErrorCodeToString(TC9_ERROR_INIT_FAILED), "Initialization failed");
    EXPECT_STREQ(ErrorCodeToString(TC9_ERROR_NO_HANDLER), "No handler registered");
    EXPECT_STREQ(ErrorCodeToString(TC9_ERROR_UNKNOWN), "Unknown error");
}

TEST(ErrorTest, TC9ExceptionBasic) {
    TC9Exception ex(TC9_ERROR_PLAYER_NOT_FOUND, "Player 123 not found");

    EXPECT_EQ(ex.code(), TC9_ERROR_PLAYER_NOT_FOUND);
    EXPECT_STREQ(ex.what(), "Player 123 not found");
}

TEST(ErrorTest, ExceptionToErrorCode) {
    TC9Exception tc9_ex(TC9_ERROR_CONNECTION_FAILED, "Connection failed");
    std::runtime_error std_ex("Standard error");

    EXPECT_EQ(ExceptionToErrorCode(tc9_ex), TC9_ERROR_CONNECTION_FAILED);
    EXPECT_EQ(ExceptionToErrorCode(std_ex), TC9_ERROR_UNKNOWN);
}
