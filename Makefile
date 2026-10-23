all: build-docker-image


#####################################################################################
# BUILD targets
#####################################################################################

# non-containerized build of the backend -- requires you to have go installed:
build-backend:
	@echo "Assuming GO is already installed -- see https://golang.org/doc/install if that's not the case"
	cd backend && \
		go mod tidy
	cd backend && \
		go build -o bin/backend . 
	@echo "Assuming golangci-lint is already installed -- see https://golangci-lint.run/usage/install/#installing-golangci-lint if that's not the case"
	cd backend && \
		golangci-lint run
	cd backend && \
		go test -v -cover ./...

fmt-backend:
	cd backend && \
		go fmt ./...
	# required by the gofumpt linter:
	#cd backend && \
	#	gofumpt -l -w -extra .
	cd backend && \
		golangci-lint run --fix

build-frontend:
	@echo "Assuming YARN is already installed -- see https://yarnpkg.com/getting-started/install if that's not the case"
	cd frontend/ && \
		yarn
	@echo "Assuming SASS is already installed -- see https://sass-lang.com/install if that's not the case"
	# transpile the SCSS -> CSS
	cd frontend && \
		sass scss/dnsmasq-dhcp.scss libs/dnsmasq-dhcp.css

DART_SASS_VERSION=1.87.0
#DART_ARCH:=linux-x64-musl
DART_ARCH:=linux-x64

install-dart-sass:
	rm -rf dart-sass
	wget https://github.com/sass/dart-sass/releases/download/$(DART_SASS_VERSION)/dart-sass-$(DART_SASS_VERSION)-$(DART_ARCH).tar.gz && \
		tar -xzf dart-sass-$(DART_SASS_VERSION)-$(DART_ARCH).tar.gz && \
		rm dart-sass-$(DART_SASS_VERSION)-$(DART_ARCH).tar.gz 
	dart-sass/sass --version
	dart-sass/sass frontend/scss/dnsmasq-dhcp.scss frontend/libs/dnsmasq-dhcp.css

INPUT_SCSS:=$(shell pwd)/frontend/scss/
OUTPUT_CSS:=$(shell pwd)/frontend

build-css-live:
	docker run -v $(INPUT_SCSS):/sass/ -v $(OUTPUT_CSS):/css/ -it michalklempa/dart-sass:latest


#####################################################################################
# DOCKER targets
#####################################################################################

# NOTE: the architecture "armhf" (ARM v6) is excluded from the list because Go toolchain is not available there
ARCH:=--amd64 --aarch64
ifeq ($(FAST),1)
# pick just 1 arch instead of all, to speedup
ARCH:=--amd64
endif
IMAGETAG:=$(shell yq .image config.yaml  | sed 's@{arch}@amd64@g')

BACKEND_SOURCE_CODE_FILES:=$(shell find backend/ -type f -name '*.go')
ROOTFS_FILES:=$(shell find rootfs/ -type f)

build-docker-image:
	# The new HomeAssistant Github actions since 2026 are basically equivalent to a 
	# "docker buildx build --platform linux/amd64,linux/arm64" command, so we can use 
	# a single-arch image for a quick local test:
	sudo docker buildx build \
		-t $(IMAGETAG):localtest \
		.

TEST_CONTAINER_NAME:=dnsmasq-dhcp-test
DOCKER_RUN_OPTIONS:= \
	-v $(shell pwd)/test-options.json:/data/options.json \
	-v $(shell pwd)/config.yaml:/opt/bin/addon-config.yaml \
	-v $(shell pwd)/test-leases.leases:/data/dnsmasq.leases \
	-v $(shell pwd)/test-db.sqlite3:/data/trackerdb.sqlite3 \
	-v $(shell pwd)/backend:/app \
	-v $(shell pwd)/frontend/index.templ.html:/opt/web/templates/index.templ.html \
	-v $(shell pwd)/frontend/libs/dnsmasq-dhcp.js:/opt/web/static/dnsmasq-dhcp.js \
	-v $(shell pwd)/frontend/libs/dnsmasq-dhcp.css:/opt/web/static/dnsmasq-dhcp.css \
	-v $(shell pwd)/rootfs/opt/bin/dnsmasq-dhcp-script.sh:/opt/bin/dnsmasq-dhcp-script.sh \
	-e LOCAL_TESTING=1 \
	--cap-add NET_ADMIN \
	--network host


#####################################################################################
# Manual test targets
#####################################################################################

# when using the 'test-docker-image' target it's normal to see messages like
#    "Something went wrong contacting the API"
# at startup of the docker container... the reason is that the startup scripts
# will try to reach to HomeAssistant Supervisor which is not running...
test-docker-image:
	$(MAKE) FAST=1 build-docker-image
	@echo
	@echo "Starting container of image $(IMAGETAG):localtest" 
	@echo "Point your browser at http://localhost:8976"
	@echo
	@echo "Starting container of image ${IMAGETAG}:localtest" 
	-docker stop $(TEST_CONTAINER_NAME) 2>/dev/null || true
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


#####################################################################################
# Automated integration tests (docker compose; see tests/integration/README.md)
#####################################################################################

IT_COMPOSE:=ADDON_IMAGE=$(IMAGETAG):localtest docker compose -f tests/integration/docker-compose.yml

# NOTE: 'compose run' is used instead of 'up --abort-on-container-exit' because the DHCP client
# containers are expected to exit (successfully) and that would abort/kill the 'tests' container (exit 137)
# requires the docker image $(IMAGETAG):localtest, built by 'make FAST=1 build-docker-image'
test-integration:
	$(IT_COMPOSE) run --build --rm tests; \
		rc=$$?; $(IT_COMPOSE) down -v; exit $$rc

# re-generate tests/integration/fixtures/dnsmasq.leases from the lease file written by the real dnsmasq
test-integration-record-fixtures:
	$(IT_COMPOSE) run --build --rm tests; \
		rc=$$?; \
		$(IT_COMPOSE) cp addon:/data/dnsmasq.leases tests/integration/fixtures/dnsmasq.leases; \
		$(IT_COMPOSE) down -v; exit $$rc

test-integration-clean:
	$(IT_COMPOSE) down -v

#####################################################################################
# More (manual) testing targets
#####################################################################################

# overwrite test-leases.leases with 3 dnsmasq-compliant entries expiring in 10 minutes
# NOTE:
# - the first 3 DHCP clients are those defined in test-options.json, so the app should
#   show the reserved IPs and the metadata for those 3 entries
# - the 4th entry is just a random one, to test how the app behaves with DHCP clients
#   that are not defined in the options.json (so they have no reserved IP and no metadata)
test-current-dhcp-leases:
	@expiry=$$(date -d '+10 minutes' +%s); \
	printf "%s aa:bb:cc:dd:ee:00 192.168.1.15 hostname1 *\n" "$$expiry" > test-leases.leases; \
	printf "%s aa:bb:cc:dd:ee:01 192.168.1.55 hostname2 *\n" "$$expiry" >> test-leases.leases; \
	printf "%s aa:bb:cc:dd:ee:02 192.168.1.56 hostname3 *\n" "$$expiry" >> test-leases.leases; \
	printf "%s aa:bb:cc:dd:ee:03 192.168.1.57 hostname4 *\n" "$$expiry" >> test-leases.leases

test-past-leases-show:
	sqlite3 test-db.sqlite3 'select * from dhcp_clients;' | column -t -s'|'

test-past-leases-describe:
	sqlite3 test-db.sqlite3 'PRAGMA table_info([dhcp_clients])'

test-past-leases-drop:
	sqlite3 test-db.sqlite3 'drop table dhcp_clients;'

# this target assumes that you launched 'test-docker-image-live' previously
test-past-leases-add-entry1:
	docker exec -ti $(TEST_CONTAINER_NAME) /opt/bin/dnsmasq-dhcp-script.sh add "dd:ee:aa:dd:00:01" "192.168.1.250" "test-entry1"

test-past-leases-add-entry2:
	docker exec -ti $(TEST_CONTAINER_NAME) /opt/bin/dnsmasq-dhcp-script.sh add "dd:ee:aa:dd:00:02" "192.168.1.251" "test-entry2"

test-past-leases-add-entry3:
	docker exec -ti $(TEST_CONTAINER_NAME) /opt/bin/dnsmasq-dhcp-script.sh add "dd:ee:aa:dd:00:03" "192.168.1.252" "test-entry3"

test-past-leases-add-entry4:
	docker exec -ti $(TEST_CONTAINER_NAME) /opt/bin/dnsmasq-dhcp-script.sh add "dd:ee:aa:dd:00:04" "192.168.1.253" ""

# NOTE:
#    docker exec -ti $(TEST_CONTAINER_NAME) /opt/bin/dnsmasq-dhcp-script.sh del "dd:ee:aa:dd:00:01" "192.168.1.250" "test-entry"
# won't work: there is no 'del' support... the only way to delete entries is to go via SQL:
test-past-leases-del-entry:

# by making the entry2 7 days older, it should be pruned by the backend from the trackerDB
# because forget_past_clients_after=1w
test-past-leases-make-entry2-very-old:
	sqlite3 test-db.sqlite3 "UPDATE dhcp_clients SET last_seen = strftime('%Y-%m-%dT%H:%M:%SZ', 'now', '-7 days') WHERE mac_addr = 'dd:ee:aa:dd:00:02';"


#####################################################################################
# Beta branch targets
#####################################################################################

recreate-beta-branch:
	@branch=$(shell git rev-parse --abbrev-ref HEAD); \
	if [ "$$branch" != "main" ]; then \
	  echo "Error: You must be on the 'main' branch to recreate beta."; \
	  exit 1; \
	fi
	git checkout -B beta
	@echo
	@echo "Editing the config.yaml to match BETA branch settings..."
	yq -i '.version = "beta"' config.yaml
	yq -i '.slug = "dnsmasq-dhcp-beta"' config.yaml
	yq -i '.name = "Dnsmasq-DHCP BETA"' config.yaml
	yq -i '.stage = "experimental"' config.yaml
	git add config.yaml
	git commit -m "Update config.yaml for BETA branch"
	@echo
	@echo "Pushing to beta branch"
	git push -f origin beta
	@echo
	@echo "Beta branch recreated from main and pushed to origin. "
	@echo "Any new feature or bugfix can be committed to the beta branch."
	@echo "Please check https://github.com/f18m/ha-addon-dnsmasq-dhcp/wiki/release-process"
	@echo

adjust-config-for-beta-branch:
	@echo
	@echo "Editing the config.yaml to match BETA branch settings..."
	yq -i '.version = "beta"' config.yaml
	yq -i '.slug = "dnsmasq-dhcp-beta"' config.yaml
	yq -i '.name = "Dnsmasq-DHCP BETA"' config.yaml
	yq -i '.stage = "experimental"' config.yaml
adjust-config-for-main-branch:
	@echo
	@echo "Editing the config.yaml to match MAIN branch settings..."
	yq -i '.version = "5.1.1"' config.yaml
	yq -i '.slug = "dnsmasq-dhcp"' config.yaml
	yq -i '.name = "Dnsmasq-DHCP"' config.yaml
	yq -i '.stage = "stable"' config.yaml
	