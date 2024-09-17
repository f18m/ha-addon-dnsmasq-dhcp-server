all: build-docker-image

ARCH:=--all
ifeq ($(FAST),1)
# pick just 1 arch instead of all, to speedup
ARCH:=--amd64
endif
IMAGETAG:=$(shell yq .image config.yaml  | sed 's@{arch}@amd64@g')

BACKEND_SOURCE_CODE_FILES:=$(shell find dhcp-clients-webapp-backend/ -type f -name '*.go')
ROOTFS_FILES:=$(shell find rootfs/ -type f)

.docker-image: $(BACKEND_SOURCE_CODE_FILES) $(ROOTFS_FILES)
	docker run \
		--rm \
		--privileged \
		-v ~/.docker:/root/.docker \
		-v /var/run/docker.sock:/var/run/docker.sock:ro \
		-v $(shell pwd):/data \
		ghcr.io/home-assistant/amd64-builder \
		$(ARCH) \
		--target /data \
		--version localtest \
		--self-cache \
		--test
	touch .docker-image

build-docker-image:
	$(MAKE) .docker-image

# non-containerized build of the backend -- requires you to have go installed:
build-backend:
	cd dhcp-clients-webapp-backend && \
		go build -o bin/backend . 

test-docker-image: 
	$(MAKE) FAST=1 .docker-image
	@echo "Starting container of image ${IMAGETAG}:localtest" 
	docker run \
		--rm \
		-v $(shell pwd)/test-options.json:/data/options.json \
		--cap-add NET_ADMIN \
		-p 8100:8100 \
		${IMAGETAG}:localtest
