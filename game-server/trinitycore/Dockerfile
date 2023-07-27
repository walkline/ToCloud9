FROM golang:1.19 AS build-sidecar

# runs from project directory
WORKDIR $GOPATH/src/github.com/walkline/ToCloud9

COPY .. .

RUN go mod download

RUN go build -o libsidecar.dylib -buildmode=c-shared ./game-server/libsidecar/

FROM ubuntu:22.04

ENV DEBIAN_FRONTEND=noninteractive

# runs from project directory
WORKDIR /

RUN apt-get update && apt-get install -yq git clang cmake make gcc g++ libmysqlclient-dev libssl-dev libbz2-dev libreadline-dev libncurses-dev libboost-all-dev mysql-server p7zip
RUN update-alternatives --install /usr/bin/cc cc /usr/bin/clang 100
RUN update-alternatives --install /usr/bin/c++ c++ /usr/bin/clang 100

RUN mkdir repo
RUN cd repo && git init && \
    git remote add origin https://github.com/TrinityCore/TrinityCore.git && \
    git fetch --depth 1 origin e11613eeb5dcad16f5908c9c7dd8109baabe9272 && \
    git checkout FETCH_HEAD

COPY game-server/trinitycore/e11613eeb5.diff ./repo/e11613eeb5.diff

RUN cd repo && git apply --reject --ignore-space-change --ignore-whitespace e11613eeb5.diff

COPY --from=build-sidecar ./go/src/github.com/walkline/ToCloud9/libsidecar.dylib ./repo/dep/libsidecar/libsidecar.dylib
COPY --from=build-sidecar ./go/src/github.com/walkline/ToCloud9/libsidecar.h ./repo/dep/libsidecar/include/libsidecar.h
COPY --from=build-sidecar ./go/src/github.com/walkline/ToCloud9/game-server/libsidecar/events-guild.h ./repo/dep/libsidecar/include/events-guild.h
COPY --from=build-sidecar ./go/src/github.com/walkline/ToCloud9/game-server/libsidecar/player-items-api.h ./repo/dep/libsidecar/include/player-items-api.h
COPY --from=build-sidecar ./go/src/github.com/walkline/ToCloud9/game-server/libsidecar/player-money-api.h ./repo/dep/libsidecar/include/player-money-api.h
COPY --from=build-sidecar ./go/src/github.com/walkline/ToCloud9/game-server/libsidecar/player-interactions-api.h ./repo/dep/libsidecar/include/player-interactions-api.h

WORKDIR /repo

RUN mkdir bin && cd bin && \
    cmake .. -DWITH_WARNINGS=1 -DWITH_COREDEBUG=0 -DUSE_COREPCH=1 -DUSE_SCRIPTPCH=1 -DTOOLS=0 -DSCRIPTS=static -DSERVERS=1 -DCMAKE_INSTALL_PREFIX=/repo/bin -DBUILD_TESTING=0

RUN cp dep/libsidecar/libsidecar.dylib /usr/lib/libsidecar.so

WORKDIR /repo/bin

RUN make -j 8 -k
RUN make install

WORKDIR /repo/bin/src/server/worldserver

ENTRYPOINT ["./worldserver"]
