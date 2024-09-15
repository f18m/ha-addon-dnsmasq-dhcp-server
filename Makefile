
all: build-docker-image


ARCH:=--all
ifeq ($(FAST),1)
ARCH:=--amd64
endif

build-docker-image:
	docker run \
		--rm \
		--privileged \
		-v ~/.docker:/root/.docker \
		-v $(shell pwd):/data \
		ghcr.io/home-assistant/amd64-builder \
		$(ARCH) \
		--target /data \
		--test

# non-containerized build of the backend -- requires you to have go installed:
build-backend:
	cd dhcp-clients-webapp-backend && go build .

# non-containerized build of the frontend -- requires you to have npm/angular installed:
build-frontend:
	cd dhcp-clients-webapp-frontend && ng build --configuration=production
	@echo
	@echo
	@echo "Now starting an nginx instance to simulate Hassio Ingress"
	@echo
	@echo
	sed 's@__ROOT__@$(shell pwd)@g' testing/nginx.conf.template >/tmp/nginx.conf
	sudo nginx -c /tmp/nginx.conf
