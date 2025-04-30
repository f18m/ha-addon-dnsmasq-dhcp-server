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

fmt-backend:
	cd dhcp-clients-webapp-backend && \
		go fmt ./...
	# required by the gofumpt linter:
	cd dhcp-clients-webapp-backend && \
		gofumpt -l -w -extra .

TEST_CONTAINER_NAME:=dnsmasq-dhcp-test
DOCKER_RUN_OPTIONS:= \
	-v $(shell pwd)/test-options.json:/data/options.json \
	-v $(shell pwd)/config.yaml:/opt/bin/addon-config.yaml \
	-v $(shell pwd)/test-leases.leases:/data/dnsmasq.leases \
	-v $(shell pwd)/test-db.sqlite3:/data/trackerdb.sqlite3 \
	-v $(shell pwd)/test-startepoch:/data/startepoch \
	-v $(shell pwd)/dhcp-clients-webapp-backend:/app \
	-v $(shell pwd)/dhcp-clients-webapp-backend/templates:/opt/web/templates/ \
	-v $(shell pwd)/rootfs/opt/bin/dnsmasq-dhcp-script.sh:/opt/bin/dnsmasq-dhcp-script.sh \
	-e LOCAL_TESTING=1 \
	--cap-add NET_ADMIN \
	--network host

# when using the 'test-docker-image' target it's normal to see messages like
#    "Something went wrong contacting the API"
# at startup of the docker container... the reason is that the startup scripts
# will try to reach to HomeAssistant Supervisor which is not running...
test-docker-image: 
	$(MAKE) FAST=1 build-docker-image
	@echo "Starting container of image ${IMAGETAG}:localtest" 
	docker run \
		--rm \
		--name $(TEST_CONTAINER_NAME) \
		${DOCKER_RUN_OPTIONS} \
		${IMAGETAG}:localtest

# NOTE: in the HTTP link below the port is actually the one in test-options.json, and currently it's 8976
test-docker-image-live: 
	sudo docker build -f Dockerfile.live -t debug-image-live .
	@echo
	@echo "Starting container of image debug-image-live" 
	@echo "Point your browser at http://localhost:8976"
	@echo
	docker run \
		--rm \
		--name $(TEST_CONTAINER_NAME) \
		${DOCKER_RUN_OPTIONS} \
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
	docker exec -ti $(TEST_CONTAINER_NAME) /opt/bin/dnsmasq-dhcp-script.sh add "00:11:22:33:44:57" "192.168.1.250" "test-entry"

test-database-add-entry2:
	docker exec -ti $(TEST_CONTAINER_NAME) /opt/bin/dnsmasq-dhcp-script.sh add "aa:bb:cc:dd:ee:01" "192.168.1.251" "test-entry2"

# NOTE:
#    docker exec -ti $(TEST_CONTAINER_NAME) /opt/bin/dnsmasq-dhcp-script.sh del "00:11:22:33:44:57" "192.168.1.250" "test-entry"
# won't work: there is no 'del' support... the only way to 
test-database-del-entry:
	sqlite3 test-db.sqlite3 "DELETE FROM dhcp_clients WHERE mac_addr = '00:11:22:33:44:57';"
