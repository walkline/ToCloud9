FROM golang:1.16 AS build-sidecar

# runs from project directory
WORKDIR /

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o libsidecar.dylib -buildmode=c-shared game-server/libsidecar/lib.go


FROM ubuntu:20.10

# runs from project directory
WORKDIR /

RUN apt-get update && apt-get install -yq libboost-all-dev g++-8 cmake libmariadbclient-dev libssl-dev libbz2-dev libreadline-dev libncurses-dev mariadb-server p7zip libmariadb-client-lgpl-dev-compat git
RUN update-alternatives --install /usr/bin/gcc gcc /usr/bin/gcc-8 100 --slave /usr/bin/g++ g++ /usr/bin/g++-8

RUN mkdir repo
RUN cd repo && git init && \
    git remote add origin https://github.com/TrinityCore/TrinityCore.git && \
    git fetch --depth 1 origin 1ddd9dc19cc1df1a1ab8c6123283999f9dea6760 && \
    git checkout FETCH_HEAD

COPY game-server/trinitycore/31ea74b96e.diff ./repo/31ea74b96e.diff

RUN cd repo && git apply 31ea74b96e.diff

COPY --from=build-sidecar ./libsidecar.dylib ./repo/dep/libsidecar/libsidecar.dylib
COPY --from=build-sidecar ./libsidecar.h ./repo/dep/libsidecar/include/libsidecar.h

WORKDIR /repo

RUN mkdir bin && cd bin && \
    cmake .. -DWITH_WARNINGS=1 -DWITH_COREDEBUG=0 -DUSE_COREPCH=1 -DUSE_SCRIPTPCH=1 -DTOOLS=0 -DSCRIPTS=static -DSERVERS=1 -DCMAKE_INSTALL_PREFIX=/repo/bin -DBUILD_TESTING=0
RUN cd bin && make -j 4 -k && make install

RUN cp dep/libsidecar/libsidecar.dylib bin/src/server/libsidecar.dylib
#RUN cp -r bin/src/server/scripts bin/src/server/worldserver/scripts

WORKDIR /repo/bin/src/server/worldserver

ENTRYPOINT ["./worldserver"]
