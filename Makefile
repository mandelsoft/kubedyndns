PROJECT=github.com/mandelsoft/kubedyndns
VERSION=$(shell cat VERSION)

RELEASE                     := true
NAME                        := coredns
NAMES                       := coredns
REPOSITORY                  := github.com/mandelsoft/kubedyndns
REGISTRY                    :=
IMAGEORG                    := mandelsoft
IMAGE_PREFIX                := $(REGISTRY)$(IMAGEORG)
REPO_ROOT                   := $(shell dirname $(realpath $(lastword $(MAKEFILE_LIST))))
HACK_DIR                    := $(REPO_ROOT)/hack
VERSION                     := $(shell cat "$(REPO_ROOT)/VERSION")
COMMIT                      := $(shell git rev-parse HEAD)
ifeq ($(RELEASE),true)
IMAGE_TAG                   := $(VERSION)
else
IMAGE_TAG                   := $(VERSION)-$(COMMIT)
endif
VERSION_VAR                 := github.com/gardener/controller-manager-library/pkg/controllermanager.Version
#LD_FLAGS                   := "-w -X $(VERSION_VAR)=$(IMAGE_TAG)"
LD_FLAGS                    := 
.PHONY: all
ifeq ($(RELEASE),true)
all: generate release
else
all: generate dev
endif


.PHONY: check
check:
	@.ci/check

.PHONY: dev
dev:
	for name in $(NAMES); do \
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GO111MODULE=on go install \
	    $(LD_FLAGS) \
	    ./cmds/$$name; \
	done

.PHONY: coredns-dev
coredns-dev:
	for name in coredns; do \
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GO111MODULE=on go install \
	    $(LD_FLAGS) \
	    ./cmds/$$name; \
	done

.PHONY: build
build:
	for name in $(NAMES); do \
	CGO_ENABLED=0 GO111MODULE=on go build -o $$name \
	    $(LD_FLAGS) \
	    ./cmds/$$name; \
	done

.PHONY: release-all
release-all: generate release

.PHONY: release
release:
	for name in $(NAMES); do \
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GO111MODULE=on go install \
	    -a \
	    $(LD_FLAGS) \
	    ./cmds/$$name; \
	done

.PHONY: coredns-release
coredns-release:
	for name in coredns; do \
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GO111MODULE=on go install \
	    -a \
	    $(LD_FLAGS) \
	    ./cmds/$$name; \
	done

.PHONY: test
test:
	GO111MODULE=on go test  ./...

.PHONY: generate
generate:
	@go generate ./...


### Docker commands

.PHONY: docker-login
docker-login:
	@gcloud auth activate-service-account --key-file .kube-secrets/gcr/gcr-readwrite.json

.PHONY: images-dev
images-dev:
	for name in $(NAMES); do \
	docker build -t $(IMAGE_PREFIX)/$$name:$(VERSION)-dev-$(COMMIT) -t $(IMAGE_PREFIX)/$$name:latest -f Dockerfile -m 6g --build-arg TARGETS=$$name-dev --build-arg NAME=$$name --target image .; \
	done

.PHONY: images-dev-push
images-dev-push: images-dev
	for name in $(NAMES); do \
	docker push $(IMAGE_PREFIX)/$$name:latest; \
	done

.PHONY: images-release
images-release:
	for name in $(NAMES); do \
	docker build -t $(IMAGE_PREFIX)/$$name:$(VERSION) -t $(IMAGE_PREFIX)/$$name:latest -f Dockerfile -m 6g --build-arg TARGETS=$$name-release --build-arg NAME=$$name --target image .; \
	done

.PHONY: images-release-all
images-release-all:
	for name in $(NAMES); do \
	docker build -t $(IMAGE_PREFIX)/$$name:$(VERSION) -t $(IMAGE_PREFIX)/$$name:latest -f Dockerfile -m 6g --build-arg TARGETS=release-all --build-arg NAME=$$name --target image .; \
	done

