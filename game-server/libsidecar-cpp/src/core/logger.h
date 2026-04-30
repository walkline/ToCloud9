#ifndef TC9_LOGGER_H
#define TC9_LOGGER_H

#include <spdlog/spdlog.h>
#include <string>

namespace tc9 {

void InitLogger(const std::string& level);
void ShutdownLogger();

}  // namespace tc9

#endif  // TC9_LOGGER_H
