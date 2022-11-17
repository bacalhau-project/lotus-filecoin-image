IMAGE ?= ghcr.io/bacalhau-project/lotus-filecoin-image
TAG ?= latest

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
