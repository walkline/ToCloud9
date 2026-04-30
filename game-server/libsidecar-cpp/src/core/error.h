#ifndef TC9_ERROR_H
#define TC9_ERROR_H

#include "libsidecar/tc9_types.h"
#include <exception>
#include <string>

namespace tc9 {

class TC9Exception : public std::exception {
public:
    explicit TC9Exception(TC9ErrorCode code, const std::string& message)
        : code_(code), message_(message) {}

    TC9ErrorCode code() const { return code_; }
    const char* what() const noexcept override { return message_.c_str(); }

private:
    TC9ErrorCode code_;
    std::string message_;
};

TC9ErrorCode ExceptionToErrorCode(const std::exception& e);
const char* ErrorCodeToString(TC9ErrorCode code);

}  // namespace tc9

#endif  // TC9_ERROR_H
