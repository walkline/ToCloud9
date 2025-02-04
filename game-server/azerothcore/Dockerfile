FROM golang:1.23 AS build-sidecar

# runs from project directory
WORKDIR $GOPATH/src/github.com/walkline/ToCloud9

COPY .. .

ENV CGO_ENABLED=1

RUN go mod download

RUN go build -o libsidecar.so -buildmode=c-shared ./game-server/libsidecar/

FROM ubuntu:22.04 AS build-core

ENV DEBIAN_FRONTEND=noninteractive

# runs from project directory
WORKDIR /

RUN apt-get update || true && \
    apt-get install -y --reinstall ubuntu-keyring && \
    apt-get update

RUN apt-get install -yq git clang cmake make gcc g++ libmysqlclient-dev libssl-dev libbz2-dev libreadline-dev libncurses-dev libboost-all-dev mysql-server p7zip
RUN update-alternatives --install /usr/bin/cc cc /usr/bin/clang 100
RUN update-alternatives --install /usr/bin/c++ c++ /usr/bin/clang 100

RUN mkdir repo
RUN cd repo && git init && \
    git remote add origin https://github.com/walkline/azerothcore-wotlk.git && \
    git fetch --depth 1 origin ea80974765a710148dbfbbf9053a1e08af982702  && \
    git checkout FETCH_HEAD

COPY --from=build-sidecar ./go/src/github.com/walkline/ToCloud9/libsidecar.so ./repo/deps/libsidecar/libsidecar.so
COPY --from=build-sidecar ./go/src/github.com/walkline/ToCloud9/libsidecar.h ./repo/deps/libsidecar/include/libsidecar.h
COPY --from=build-sidecar ./go/src/github.com/walkline/ToCloud9/game-server/libsidecar/events-guild.h ./repo/deps/libsidecar/include/events-guild.h
COPY --from=build-sidecar ./go/src/github.com/walkline/ToCloud9/game-server/libsidecar/events-group.h ./repo/deps/libsidecar/include/events-group.h
COPY --from=build-sidecar ./go/src/github.com/walkline/ToCloud9/game-server/libsidecar/events-servers-registry.h ./repo/deps/libsidecar/include/events-servers-registry.h
COPY --from=build-sidecar ./go/src/github.com/walkline/ToCloud9/game-server/libsidecar/player-items-api.h ./repo/deps/libsidecar/include/player-items-api.h
COPY --from=build-sidecar ./go/src/github.com/walkline/ToCloud9/game-server/libsidecar/player-money-api.h ./repo/deps/libsidecar/include/player-money-api.h
COPY --from=build-sidecar ./go/src/github.com/walkline/ToCloud9/game-server/libsidecar/player-interactions-api.h ./repo/deps/libsidecar/include/player-interactions-api.h

WORKDIR /repo

RUN cd bin && \
    cmake .. -DUSE_REAL_LIBSIDECAR=ON \
        -DBoost_USE_STATIC_LIBS=ON \
        -DWITH_WARNINGS=1 \
        -DWITH_COREDEBUG=0 \
        -DUSE_COREPCH=1 \
        -DUSE_SCRIPTPCH=1 \
        -DTOOLS=0 \
        -DSCRIPTS=static \
        -DSERVERS=1 \
        -DCMAKE_BUILD_TYPE=RelWithDebInfo \
        -DCMAKE_INSTALL_PREFIX=/repo/bin \
        -DBUILD_TESTING=0

RUN cp deps/libsidecar/libsidecar.so /usr/lib/libsidecar.so

WORKDIR /repo/bin

RUN cmake --build . --config "RelWithDebInfo" -j $(($(nproc) + 1))
RUN cmake --install . --config "RelWithDebInfo"

FROM ubuntu:22.04

RUN apt-get update && \
    apt-get install -y --no-install-recommends \
      libmysqlclient21 libreadline8 \
      gettext-base default-mysql-client && \
    rm -rf /var/lib/apt/lists/*

COPY --from=build-sidecar ./go/src/github.com/walkline/ToCloud9/libsidecar.so /usr/lib/libsidecar.so

COPY --from=build-core /repo/bin/etc/worldserver.conf.dist /repo/bin/etc/worldserver.conf
COPY --from=build-core /repo/bin/bin/worldserver /repo/bin/worldserver
COPY --from=build-core /repo/data/sql/ /repo/data/sql/

WORKDIR /repo/bin

ENTRYPOINT ["./worldserver"]
