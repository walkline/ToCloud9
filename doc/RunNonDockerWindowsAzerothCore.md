# Build and Run on Windows (Native) with AzerothCore

This guide shows how to run ToCloud9 on Windows natively, with AzerothCore compiled on Windows using libsidecar-cpp.

### Prerequisites

* [Visual Studio 2022](https://visualstudio.microsoft.com/downloads/) with C++ Desktop Development workload
* [CMake](https://cmake.org/download/) (3.20 or higher)
* [Git](https://git-scm.com/download/win)
* [Go](https://go.dev/dl/) (for ToCloud9 services)
* [Redis for Windows](https://github.com/redis-windows/redis-windows) or [Memurai](https://www.memurai.com/)
* [NATS Server](https://docs.nats.io/running-a-nats-service/introduction/installation#windows)
* [MySQL Server](https://dev.mysql.com/downloads/mysql/8.0.html) with configured AzerothCore databases

### Build ToCloud9 Services

1. Clone ToCloud9 repository:
```cmd
git clone https://github.com/walkline/ToCloud9.git
cd ToCloud9
```

2. Build Go services using PowerShell or Command Prompt:
```cmd
go build -o bin\authserver.exe apps\authserver\cmd\authserver\main.go
go build -o bin\charserver.exe apps\charserver\cmd\charserver\main.go
go build -o bin\chatserver.exe apps\chatserver\cmd\chatserver\main.go
go build -o bin\game-load-balancer.exe apps\game-load-balancer\cmd\game-load-balancer\main.go
go build -o bin\servers-registry.exe apps\servers-registry\cmd\servers-registry\main.go
go build -o bin\guidserver.exe apps\guidserver\cmd\guidserver\main.go
go build -o bin\guildserver.exe apps\guildserver\cmd\guildserver\main.go
go build -o bin\groupserver.exe apps\groupserver\cmd\groupserver\main.go
go build -o bin\mailserver.exe apps\mailserver\cmd\mailserver\main.go
```

### Build libsidecar-cpp

#### Option 1: Download Pre-built Binary (Recommended)

1. Download the appropriate version from the [GitHub releases page](https://github.com/walkline/ToCloud9/releases):
   - **Visual Studio 2022**: Download `libsidecar-cpp-windows-x64-v143.zip`
   - **Visual Studio 2019**: Download `libsidecar-cpp-windows-x64-v142.zip`

   **Important**: Choose the version matching your Visual Studio installation. Using the wrong version will cause linking errors or runtime crashes.

2. Extract the archive to a convenient location (e.g., `C:\libsidecar-cpp`).

3. You'll have:
   - `lib\libsidecar.dll` - the shared library
   - `lib\libsidecar.lib` - the import library for linking
   - `include\*.h` - header files
   - `README.txt` - build information and requirements

**Note**: All dependencies (gRPC, NATS, prometheus, spdlog, etc.) are statically linked into `libsidecar.dll`. The builds use static MSVC runtime (/MT), so you don't need to install Visual C++ Redistributable separately.

#### Option 2: Build from Source

1. Open **x64 Native Tools Command Prompt for VS 2022** (important - must be x64 tools).

2. Navigate to the libsidecar-cpp directory:
```cmd
cd game-server\libsidecar-cpp
```

3. Build with CMake:
```cmd
mkdir build
cd build
cmake .. -G "Visual Studio 17 2022" -A x64 -DCMAKE_BUILD_TYPE=Release -DBUILD_TESTS=OFF
cmake --build . --config Release -j %NUMBER_OF_PROCESSORS%
```

4. The built artifacts will be in:
   - `build\Release\libsidecar.dll` - the shared library
   - `build\Release\libsidecar.lib` - the import library
   - `..\include\*.h` - header files

### Build AzerothCore with libsidecar-cpp

1. Clone AzerothCore with cluster mode support:
```cmd
cd C:\dev
git clone --branch cluster-mode https://github.com/walkline/azerothcore-wotlk.git
cd azerothcore-wotlk
```

2. Copy libsidecar-cpp files to AzerothCore:

**If using pre-built binary:**
```cmd
xcopy /E /I C:\libsidecar-cpp\lib\libsidecar.dll deps\libsidecar\
xcopy /E /I C:\libsidecar-cpp\lib\libsidecar.lib deps\libsidecar\
xcopy /E /I C:\libsidecar-cpp\include deps\libsidecar\include
```

**If built from source:**
```cmd
copy C:\path\to\ToCloud9\game-server\libsidecar-cpp\build\Release\libsidecar.dll deps\libsidecar\
copy C:\path\to\ToCloud9\game-server\libsidecar-cpp\build\Release\libsidecar.lib deps\libsidecar\
xcopy /E /I C:\path\to\ToCloud9\game-server\libsidecar-cpp\include deps\libsidecar\include
```

3. Build AzerothCore using the standard Windows build process with the additional CMake flag:
```cmd
mkdir build
cd build
cmake .. -G "Visual Studio 17 2022" -A x64 -DUSE_REAL_LIBSIDECAR=ON
cmake --build . --config RelWithDebInfo
```

4. Follow the [official AzerothCore Windows installation guide](https://www.azerothcore.org/wiki/windows-requirements) for the remaining setup (extracting client data, configuring database, etc.).

### Setup & Run

1. Apply database migrations from `sql/characters/mysql` folder to your `acore_characters` database.

2. Copy `config.yml.example` to `bin\config.yml` in your ToCloud9 directory.

3. Edit `bin\config.yml` and update the database connection strings:
```yaml
db:
  auth: &defaultAuthDB "dbUser:dbPassword@tcp(127.0.0.1:3306)/acore_auth"
  characters: &defaultCharactersDB "dbUser:dbPassword@tcp(127.0.0.1:3306)/acore_characters"
  world: &defaultWorldDB "dbUser:dbPassword@tcp(127.0.0.1:3306)/acore_world"
  schemaType: &defaultSchemaType "ac"
```

4. Start required services:
   - Start **MySQL Server**
   - Start **NATS Server**: Open a terminal and run `nats-server`
   - Start **Redis**: Open a terminal and run `redis-server` or start Memurai service

5. Run ToCloud9 services from the `bin` directory in this order:

First, start core services:
```cmd
start servers-registry.exe
start guidserver.exe
```

Wait a few seconds, then start the remaining services:
```cmd
start authserver.exe
start charserver.exe
start chatserver.exe
start game-load-balancer.exe
start guildserver.exe
start groupserver.exe
start mailserver.exe
```

6. Make sure `libsidecar.dll` is in your PATH or copy it next to `worldserver.exe`:
```cmd
copy C:\dev\azerothcore-wotlk\deps\libsidecar\libsidecar.dll C:\dev\azerothcore-wotlk\bin\RelWithDebInfo\
```

7. Run AzerothCore worldserver from the build directory:
```cmd
cd C:\dev\azerothcore-wotlk\bin\RelWithDebInfo
set AC_CLUSTER_ENABLED=1
set AC_WORLD_SERVER_PORT=9601
worldserver.exe
```

8. To run additional worldservers (for load balancing):
```cmd
# Second worldserver (new terminal)
cd C:\dev\azerothcore-wotlk\bin\RelWithDebInfo
set AC_CLUSTER_ENABLED=1
set AC_WORLD_SERVER_PORT=9602
set GRPC_PORT=9603
set HEALTH_CHECK_PORT=9604
worldserver.exe

# Worldserver for specific map (Icecrown Citadel)
cd C:\dev\azerothcore-wotlk\bin\RelWithDebInfo
set AC_CLUSTER_AVAILABLE_MAPS=631
set AC_CLUSTER_ENABLED=1
set AC_WORLD_SERVER_PORT=9605
set GRPC_PORT=9606
set HEALTH_CHECK_PORT=9607
worldserver.exe
```

### Troubleshooting

**Missing DLL errors:**
- Ensure `libsidecar.dll` is in the same directory as `worldserver.exe` or in your system PATH
- You may need to install [Visual C++ Redistributable](https://learn.microsoft.com/en-us/cpp/windows/latest-supported-vc-redist) if building with Visual Studio

**Port conflicts:**
- Make sure the ports specified in environment variables are not used by other applications

**Database connection errors:**
- Verify MySQL is running and credentials in `config.yml` are correct

Feel free to share your feedback in the [Discord channel](https://discord.gg/QxfBD9uGbN).
