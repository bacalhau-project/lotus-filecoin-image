# build container stage
FROM golang:1.18-buster AS build-env

RUN apt-get update -y && \
    apt-get install sudo cron git mesa-opencl-icd gcc bzr jq pkg-config clang libhwloc-dev ocl-icd-opencl-dev build-essential hwloc -y

RUN curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh -s -- -y
ENV PATH="/root/.cargo/bin:${PATH}"

ENV LOTUS_PATH="~/.lotus-local-net"
ENV LOTUS_MINER_PATH="~/.lotus-miner-local-net"
ENV CGO_CFLAGS="-D__BLST_PORTABLE__"
ENV CGO_CFLAGS_ALLOW="-D__BLST_PORTABLE__"
ENV LOTUS_SKIP_GENESIS_CHECK="_yes_"
ENV RUSTFLAGS="-C target-cpu=native -g"
ENV FFI_BUILD_FROM_SOURCE=1
ENV NETWORK="2k"
# Tag of the lotus version to build
ENV BRANCH=v1.17.2

WORKDIR /src

RUN git clone https://github.com/filecoin-project/lotus.git --depth 1 --branch $BRANCH /src/lotus
RUN cd /src/lotus && git submodule update --init --recursive
RUN cd /src/lotus && make $NETWORK

# building the healthcheck util, to know when Lotus is ready for use
FROM golang:1.18-buster AS utils

WORKDIR /src

COPY go.mod .
COPY go.sum .
RUN go mod download

COPY . .
RUN go build -trimpath -o /usr/local/bin/healthcheck ./cmd/healthcheck

# runtime container stage
FROM ubuntu:20.04
LABEL network=local
LABEL filecoin=lotus

ENV DEBIAN_FRONTEND noninteractive
ENV LOTUS_USER="lotus_user"

# create nonroot user and lotus folder
RUN adduser --uid 2000 --gecos "" --disabled-password --quiet $LOTUS_USER

COPY --from=build-env /src/lotus/lotus /usr/local/bin/lotus
COPY --from=build-env /src/lotus/lotus-gateway /usr/local/bin/lotus-gateway
COPY --from=build-env /src/lotus/lotus-shed /usr/local/bin/lotus-shed
COPY --from=build-env /src/lotus/lotus-miner /usr/local/bin/lotus-miner
COPY --from=build-env /src/lotus/lotus-seed /usr/local/bin/lotus-seed
COPY --from=build-env /etc/ssl/certs /etc/ssl/certs
COPY --from=build-env /lib/*-linux-gnu /lib/
# lotus libraries
COPY --from=build-env /lib/*-linux-gnu/libutil.so.1 \
                      /lib/*-linux-gnu/librt.so.1 \
                      /lib/*-linux-gnu/libgcc_s.so.1 \
                      /lib/*-linux-gnu/libdl.so.2 \
                      /usr/lib/*-linux-gnu/libltdl.so.7 \
                      /usr/lib/*-linux-gnu/libnuma.so.1 \
                      /usr/lib/*-linux-gnu/libhwloc.so.5 /lib/
COPY --from=build-env /usr/lib/*-linux-gnu/libOpenCL.so.1.0.0 /lib/libOpenCL.so.1

ENV LOTUS_PATH="/home/$LOTUS_USER/.lotus-local-net"
ENV LOTUS_MINER_PATH="/home/$LOTUS_USER/.lotus-miner-local-net"
ENV LOTUS_SKIP_GENESIS_CHECK="_yes_"
ENV CGO_CFLAGS_ALLOW="-D__BLST_PORTABLE__"
ENV CGO_CFLAGS="-D__BLST_PORTABLE__"

USER $LOTUS_USER
WORKDIR /home/$LOTUS_USER
RUN lotus fetch-params 2048 && \
    lotus-seed pre-seal --sector-size 2KiB --num-sectors 2
RUN lotus-seed genesis new ~/localnet.json
RUN lotus-seed genesis add-miner ~/localnet.json ~/.genesis-sectors/pre-seal-t01000.json

COPY --from=utils /usr/local/bin/healthcheck /usr/local/bin/healthcheck

COPY --chown=2000:2000 config/daemon.toml $LOTUS_PATH/config.toml
COPY --chown=2000:2000 config/miner.toml $LOTUS_MINER_PATH/config.toml
COPY --chown=0:0 scripts/run /usr/local/bin/run-lotus

HEALTHCHECK --interval=5s --timeout=2s --start-period=1m CMD ["/usr/local/bin/healthcheck"]

# API port
EXPOSE 1234/tcp

ENTRYPOINT ["/usr/local/bin/run-lotus"]
