all: build-docker-image

# NOTE: the architecture "armhf" (ARM v6) is excluded from the list because Go toolchain is not available there
ARCH:=--armv7 --amd64 --aarch64 --i386
ifeq ($(FAST),1)
# pick just 1 arch instead of all, to speedup
ARCH:=--amd64
endif
IMAGETAG:=$(shell yq .image config.yaml  | sed 's@{arch}@amd64@g')

BACKEND_SOURCE_CODE_FILES:=$(shell find dhcp-clients-webapp-backend/ -type f -name '*.go')
ROOTFS_FILES:=$(shell find rootfs/ -type f)

build-docker-image: $(BACKEND_SOURCE_CODE_FILES) $(ROOTFS_FILES)
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

# non-containerized build of the backend -- requires you to have go installed:
build-backend:
	cd dhcp-clients-webapp-backend && \
		go build -o bin/backend . 
	cd dhcp-clients-webapp-backend && \
		golangci-lint run
	cd dhcp-clients-webapp-backend && \
		go test -v -cover ./...
	

test-docker-image: 
	$(MAKE) FAST=1 build-docker-image
	@echo "Starting container of image ${IMAGETAG}:localtest" 
	docker run \
		--rm \
		-v $(shell pwd)/test-options.json:/data/options.json \
		-v $(shell pwd)/test-leases.leases:/data/dnsmasq.leases \
		-e LOCAL_TESTING=1 \
		--cap-add NET_ADMIN \
		--network host \
		${IMAGETAG}:localtest

test-docker-image-live: 
	docker build -f Dockerfile.live -t debug-image-live .
	@echo "Starting container of image debug-image-live" 
	docker run \
		--rm \
		-v $(shell pwd)/test-options.json:/data/options.json \
		-v $(shell pwd)/test-leases.leases:/data/dnsmasq.leases \
		-v $(shell pwd)/dhcp-clients-webapp-backend:/app \
		-v $(shell pwd)/dhcp-clients-webapp-backend/templates:/opt/web/templates/ \
		-e LOCAL_TESTING=1 \
		--cap-add NET_ADMIN \
		--network host \
		debug-image-live


INPUT_SCSS:=$(shell pwd)/dhcp-clients-webapp-backend/templates/scss/
OUTPUT_CSS:=$(shell pwd)/dhcp-clients-webapp-backend/templates/

build-css:
	docker run -v $(INPUT_SCSS):/sass/ -v $(OUTPUT_CSS):/css/ -it michalklempa/dart-sass:latest
		
