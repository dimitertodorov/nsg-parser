# Copyright 2017 Dimiter Todorov (dimiter.todorov@gmail.com)

GO           := GO15VENDOREXPERIMENT=1 go
FIRST_GOPATH := $(firstword $(subst :, ,$(shell $(GO) env GOPATH)))
PROMU        := $(FIRST_GOPATH)/bin/promu
pkgs          = $(shell $(GO) list ./... | grep -v /vendor/)
PREFIX                  ?= $(shell pwd)
BIN_DIR                 ?= $(shell pwd)
DOCKER_IMAGE_NAME       ?= nsg-parser
DOCKER_IMAGE_TAG        ?= $(subst /,-,$(shell git rev-parse --abbrev-ref HEAD))
GITHUB_ORG              ?= dimitertodorov
GITHUB_REPO             ?= nsg-parser
VERSION      := $(shell cat ./VERSION)

ifdef DEBUG
bindata_flags = -debug
endif

all: format build test

style:
	@echo ">> checking code style"
	@! gofmt -d $(shell find . -path ./vendor -prune -o -name '*.go' -print) | grep '^'

test-short:
	@echo ">> running short tests"
	@$(GO) test -short $(pkgs)

test:
	@echo ">> running all tests"
	@$(GO) test $(pkgs)

format:
	@echo ">> formatting code"
	@$(GO) fmt $(pkgs)

vet:
	@echo ">> vetting code"
	@$(GO) vet $(pkgs)

build: promu
	@echo ">> building binaries"
	@$(PROMU) build --prefix $(PREFIX)

tarball: promu
	@echo ">> building release tarball"
	@$(PROMU) tarball --prefix $(PREFIX) $(BIN_DIR)

crossbuild: promu
	@echo ">> cross-building release"
	@$(PROMU) crossbuild

tarballs: promu
	@echo ">> create release tarballs"
	@$(PROMU) crossbuild tarballs

convert_windows:
	@rm -rf ./.windows/*
	$(eval winFormat := "*$(VERSION)*windows*")
	$(eval winTarball := $(shell find ./.tarballs/ -type f -name $(winFormat).tar.gz))
	@echo "$(winTarball)--"
	@for f in $(winTarball); do tar -C ./.windows/ -zxf $$f && rm -f $$f; done
	$(eval winfiles := $(shell find ./.windows/ -type d -name $(winFormat)))
	@for d in $(winfiles); \
	do \
	basename=`basename $$d`; \
	echo "<< Converting $$basename to zip"; \
	zip ./.windows/$$basename.zip $$d/*; \
	mv ./.windows/$$basename.zip ./.tarballs/; \
	done

release_all: github_release
	$(eval versionFmt := "*$(VERSION)*")
	$(eval releaseFiles := $(shell find ./.tarballs/ -type f \( -name "$(versionFmt).zip" -or -name "$(versionFmt).tar.gz" \)))
	@for d in $(releaseFiles); \
	do \
	basename=`basename $$d`; \
	echo "<< Releasing $$basename"; \
	github-release upload --user $(GITHUB_ORG) --repo $(GITHUB_REPO) --tag "v$(VERSION)" --name $$basename --file $$d --replace; \
	done

promu:
	@echo ">> fetching promu"
	@GOOS=$(shell uname -s | tr A-Z a-z) \
	GOARCH=$(subst x86_64,amd64,$(patsubst i%86,386,$(shell uname -m))) \
	$(GO) get -u github.com/prometheus/promu

github_release:
	$(GO) get -u github.com/aktau/github-release

.PHONY: all style check_license format build test vet assets tarball docker promu