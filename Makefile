# Dispatch — build targets.
# Override VERSION to stamp a release: `make build VERSION=0.2.0`.
VERSION ?= dev
PKG      := github.com/iftimiemarius/dispatch
LDFLAGS  := -X $(PKG)/internal/cli.Version=$(VERSION)
BINARY   := dispatch

.PHONY: all build run test vet fmt tidy clean install install-skill uninstall-skill

all: build

## build: compile the dispatch binary into ./$(BINARY)
build:
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) ./cmd/dispatch

## run: build and run with the given args (make run ARGS='today')
run: build
	./$(BINARY) $(ARGS)

## test: run the test suite
test:
	go test ./...

## vet: run go vet
vet:
	go vet ./...

## fmt: format Go sources
fmt:
	gofmt -s -w .

## tidy: sync dependencies
tidy:
	go mod tidy

## install: build and install to $$GOBIN (or $$GOPATH/bin)
install: build
	@mkdir -p $(shell go env GOPATH)/bin
	cp $(BINARY) $(shell go env GOPATH)/bin/$(BINARY)

## clean: remove build artifacts
clean:
	rm -f $(BINARY)

## install-skill: symlink the dispatch ZCode skill into ~/.zcode/skills (user scope)
install-skill:
	@mkdir -p $(HOME)/.zcode/skills
	@ln -sfn $(CURDIR)/skills/dispatch $(HOME)/.zcode/skills/dispatch
	@echo "Skill 'dispatch' linked to ~/.zcode/skills/dispatch (restart ZCode to load)"

## uninstall-skill: remove the dispatch ZCode skill link
uninstall-skill:
	@rm -f $(HOME)/.zcode/skills/dispatch
	@echo "Skill 'dispatch' unlinked"
