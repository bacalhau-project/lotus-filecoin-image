# lotus-filecoin-image

This repository is responsible for creating a Docker image that contains a [Lotus](https://lotus.filecoin.io) miner
running a [local network](https://lotus.filecoin.io/lotus/developers/local-network/). The purpose of the image is to
make it easier to test interacting with the [FileCoin network](https://filecoin.io) without having to use the
[mainnet](https://docs.filecoin.io/networks/overview/#mainnet).

The image is configured with a health check that will become healthy once the daemon and miner are up and running. A
token can be found within the container at `/home/lotus_user/.lotus-local-net/token`, which can be used to access the
API from outside the container.
