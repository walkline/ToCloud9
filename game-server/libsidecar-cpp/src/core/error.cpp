#include "error.h"

namespace tc9 {

TC9ErrorCode ExceptionToErrorCode(const std::exception& e) {
    const TC9Exception* tc9_ex = dynamic_cast<const TC9Exception*>(&e);
    if (tc9_ex) {
        return tc9_ex->code();
    }
    return TC9_ERROR_UNKNOWN;
}

const char* ErrorCodeToString(TC9ErrorCode code) {
    switch (code) {
        case TC9_ERROR_SUCCESS:
            return "Success";
        case TC9_ERROR_INIT_FAILED:
            return "Initialization failed";
        case TC9_ERROR_NO_HANDLER:
            return "No handler registered";
        case TC9_ERROR_PLAYER_NOT_FOUND:
            return "Player not found";
        case TC9_ERROR_NO_INVENTORY_SPACE:
            return "No inventory space";
        case TC9_ERROR_UNKNOWN_TEMPLATE:
            return "Unknown template";
        case TC9_ERROR_FAILED_TO_CREATE_ITEM:
            return "Failed to create item";
        case TC9_ERROR_CONNECTION_FAILED:
            return "Connection failed";
        case TC9_ERROR_UNKNOWN:
        default:
            return "Unknown error";
    }
}

}  // namespace tc9
