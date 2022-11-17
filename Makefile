IMAGE ?= ghcr.io/bacalhau-project/lotus-filecoin-image
TAG ?= latest

LOTUS_TAG := v1.18.0

.PHONY: build
build:
	docker build --build-arg BRANCH=$(LOTUS_TAG) --tag $(IMAGE):$(TAG) .

.PHONY: test
test:
	TEST_IMAGE=$(IMAGE):$(TAG) go test -count=1 -v ./tests/...

.PHONY: push
push:
	docker push $(IMAGE):$(TAG)

.PHONY: run
run:
	docker run --tty --detach --publish 1234:1234 --volume ${PWD}/testdata:/home/lotus_user/testdata --name lotus $(IMAGE):$(TAG)

.PHONY: bash
bash:
	docker exec --interactive --tty $(shell docker ps --quiet --filter=label=network=local --filter=label=filecoin=lotus) /bin/bash

.PHONY: status
status:
	docker exec --interactive --tty $(shell docker ps --quiet --filter=label=network=local --filter=label=filecoin=lotus) lotus sync status

.PHONY: token
token:
	docker exec --interactive --tty $(shell docker ps --quiet --filter=label=network=local --filter=label=filecoin=lotus) bash -c "cat ~/.lotus-local-net/token"

.PHONY: log
log:
	docker logs --follow $(shell docker ps --quiet --filter=label=network=local --filter=label=filecoin=lotus) 2>&1

.PHONY: clean
clean:
	docker ps --quiet --filter=label=network=local --filter=label=filecoin=lotus | xargs docker stop
	docker ps --all --quiet --filter=label=network=local --filter=label=filecoin=lotus | xargs docker rm --volumes
