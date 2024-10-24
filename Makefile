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

LIVE_CONTAINER_NAME:=dsnmasq-dhcp-test-live

test-docker-image-live: 
	docker build -f Dockerfile.live -t debug-image-live .
	@echo "Starting container of image debug-image-live" 
	docker run \
		--rm \
		--name $(LIVE_CONTAINER_NAME) \
		-v $(shell pwd)/test-options.json:/data/options.json \
		-v $(shell pwd)/test-leases.leases:/data/dnsmasq.leases \
		-v $(shell pwd)/test-db.sqlite3:/data/trackerdb.sqlite3 \
		-v $(shell pwd)/test-startcounter:/data/startcounter \
		-v $(shell pwd)/dhcp-clients-webapp-backend:/app \
		-v $(shell pwd)/dhcp-clients-webapp-backend/templates:/opt/web/templates/ \
		-v $(shell pwd)/rootfs/opt/bin/dnsmasq-dhcp-script.sh:/opt/bin/dnsmasq-dhcp-script.sh \
		-e LOCAL_TESTING=1 \
		--cap-add NET_ADMIN \
		--network host \
		debug-image-live


INPUT_SCSS:=$(shell pwd)/dhcp-clients-webapp-backend/templates/scss/
OUTPUT_CSS:=$(shell pwd)/dhcp-clients-webapp-backend/templates/

build-css-live:
	docker run -v $(INPUT_SCSS):/sass/ -v $(OUTPUT_CSS):/css/ -it michalklempa/dart-sass:latest

test-database-show:
	sqlite3 test-db.sqlite3 'select * from dhcp_clients;'		

test-database-drop:
	sqlite3 test-db.sqlite3 'drop table dhcp_clients;'

# this target assumes that you launched 'test-docker-image-live' previously
test-database-add-entry:
	docker exec -ti $(LIVE_CONTAINER_NAME) /opt/bin/dnsmasq-dhcp-script.sh add "00:11:22:33:44:57" "192.168.1.250" "test-entry"
	docker exec -ti $(LIVE_CONTAINER_NAME) cat /data/dnsmasq-dhcp-script.log

test-database-add-entry2:
	docker exec -ti $(LIVE_CONTAINER_NAME) /opt/bin/dnsmasq-dhcp-script.sh add "00:11:22:33:44:58" "192.168.1.251" "test-entry2"
	docker exec -ti $(LIVE_CONTAINER_NAME) cat /data/dnsmasq-dhcp-script.log

# NOTE:
#    docker exec -ti $(LIVE_CONTAINER_NAME) /opt/bin/dnsmasq-dhcp-script.sh del "00:11:22:33:44:57" "192.168.1.250" "test-entry"
# won't work: there is no 'del' support... the only way to 
test-database-del-entry:
	sqlite3 test-db.sqlite3 "DELETE FROM dhcp_clients WHERE mac_addr = '00:11:22:33:44:57';"
