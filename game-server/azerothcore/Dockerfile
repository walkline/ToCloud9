FROM --platform=${BUILDPLATFORM} golang:1.19 AS build-sidecar

# runs from project directory
WORKDIR $GOPATH/src/github.com/walkline/ToCloud9

COPY .. .

RUN go mod download

RUN go build -o libsidecar.so -buildmode=c-shared ./game-server/libsidecar/

FROM --platform=${BUILDPLATFORM} ubuntu:22.04

ENV DEBIAN_FRONTEND=noninteractive

# runs from project directory
WORKDIR /

RUN apt-get update && apt-get install -yq git clang cmake make gcc g++ libmysqlclient-dev libssl-dev libbz2-dev libreadline-dev libncurses-dev libboost-all-dev mysql-server p7zip
RUN update-alternatives --install /usr/bin/cc cc /usr/bin/clang 100
RUN update-alternatives --install /usr/bin/c++ c++ /usr/bin/clang 100

RUN mkdir repo
RUN cd repo && git init && \
    git remote add origin https://github.com/walkline/azerothcore-wotlk.git && \
    git fetch --depth 1 origin 902118f002cab4268bcdc7e2d9e55c7b3d832379 && \
    git checkout FETCH_HEAD

COPY --from=build-sidecar ./go/src/github.com/walkline/ToCloud9/libsidecar.so ./repo/deps/libsidecar/libsidecar.so
COPY --from=build-sidecar ./go/src/github.com/walkline/ToCloud9/libsidecar.h ./repo/deps/libsidecar/include/libsidecar.h
COPY --from=build-sidecar ./go/src/github.com/walkline/ToCloud9/game-server/libsidecar/events-guild.h ./repo/deps/libsidecar/include/events-guild.h
COPY --from=build-sidecar ./go/src/github.com/walkline/ToCloud9/game-server/libsidecar/player-items-api.h ./repo/deps/libsidecar/include/player-items-api.h
COPY --from=build-sidecar ./go/src/github.com/walkline/ToCloud9/game-server/libsidecar/player-money-api.h ./repo/deps/libsidecar/include/player-money-api.h
COPY --from=build-sidecar ./go/src/github.com/walkline/ToCloud9/game-server/libsidecar/player-interactions-api.h ./repo/deps/libsidecar/include/player-interactions-api.h

WORKDIR /repo

RUN cd bin && \
    cmake .. -DUSE_REAL_LIBSIDECAR=ON -DWITH_WARNINGS=1 -DWITH_COREDEBUG=0 -DUSE_COREPCH=1 -DUSE_SCRIPTPCH=1 -DTOOLS=0 -DSCRIPTS=static -DSERVERS=1 -DCMAKE_INSTALL_PREFIX=/repo/bin -DBUILD_TESTING=0

RUN cp deps/libsidecar/libsidecar.so /usr/lib/libsidecar.so

WORKDIR /repo/bin

RUN make -j 8 -k
RUN make install

WORKDIR /repo/bin/src/server/apps

ENTRYPOINT ["./worldserver"]
