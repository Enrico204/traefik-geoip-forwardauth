DOCKER_TAG := $(shell git fetch --unshallow >/dev/null 2>&1; git describe --long --tags 2>/dev/null)

.PHONY: all
all: build

.PHONY: info
info:
	$(info hub.netsplit.it/utilities/traefik-geoip-forwardauth:${DOCKER_TAG})
	@:

.PHONY: build
build:
	go generate -mod=vendor ./...
	go build -mod=vendor -ldflags "-extldflags \"-static\"" -a -installsuffix cgo -o traefik-geoip-forwardauth .

.PHONY: docker
docker:
	docker build \
		-t hub.netsplit.it/utilities/traefik-geoip-forwardauth:${DOCKER_TAG} \
		.

.PHONY: push
push:
	docker push hub.netsplit.it/utilities/traefik-geoip-forwardauth:${DOCKER_TAG}
	docker tag hub.netsplit.it/utilities/traefik-geoip-forwardauth:${DOCKER_TAG} hub.netsplit.it/utilities/traefik-geoip-forwardauth:latest
	docker push hub.netsplit.it/utilities/traefik-geoip-forwardauth:latest

.PHONY: push-dockerhub
push-dockerhub:
	docker tag hub.netsplit.it/utilities/traefik-geoip-forwardauth:${DOCKER_TAG} enrico204/traefik-geoip-forwardauth:${DOCKER_TAG}
	docker push enrico204/traefik-geoip-forwardauth:${DOCKER_TAG}
	docker tag enrico204/traefik-geoip-forwardauth:${DOCKER_TAG} enrico204/traefik-geoip-forwardauth:latest
	docker push enrico204/traefik-geoip-forwardauth:latest

.PHONY: test
test:
	go clean -testcache
	go test ./... -mod=vendor
	golangci-lint run
	go list -mod=mod -u -m -json all | go-mod-outdated -update -direct

.PHONY: godoc
godoc:
	$(info Open http://localhost:6060/pkg/${GO_MODULE}/)
ifdef ($GOROOT,)
	godoc
else
	godoc -goroot /usr/share/go
endif
