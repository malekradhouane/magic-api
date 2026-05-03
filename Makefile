.DEFAULT_GOAL := all
MAKEFLAGS += --no-print-directory

# DO NOT EDIT BY HAND
# To set a different name, use:
# $ export ENVIRONMENT="new_env"
# from your shell.

# Where to put all builded binaries
BINDIR ?= $(CURDIR)/bin

MAGIC_BIN = $(BINDIR)/magic
MAGIC_CONFIG_ROOT_DIR ?= $(CURDIR)
MAGIC_SOURCES = $(shell find $(CURDIR) -type f -name "*.go" -or -name "*.sql")

SWAGGER_OUTPUT = $(CURDIR)/docs

STOREINIT_BIN = $(BINDIR)/storeinit

GO = go
GOMODULE = $(shell head -1 go.mod | cut -d" " -f2)
LDFLAGS = -X 'main.GitTag=$(shell git describe --tags)'
GCFLAGS =

.PHONY: all
all: magic storeinit

.PHONY: storeinit
storeinit:
	@echo "Building $@"
	@cd cmd/_$@ && $(MAKE)

.PHONY: magic
magic: $(MAGIC_BIN)


$(MAGIC_BIN): $(MAGIC_SOURCES)

	@echo "Building Magic"

	@echo "    Generating swagger files"
	@swag init --quiet --dir ./cmd/magic,./handler,./api,./pkg/interfaces,./service --parseDependency --parseInternal --parseDepth 3 --output $(SWAGGER_OUTPUT) --outputTypes yaml,go || echo "    [warn] swagger generation skipped"

	@echo "    Compiling"
	@echo
	@CGO_ENABLED=0 $(GO) build \
	-ldflags "$(LDFLAGS)" \
	-gcflags "$(GCFLAGS)" \
	-o $(MAGIC_BIN) cmd/magic/*.go

.PHONY: run
run: $(MAGIC_BIN)
	@echo "Loading environment variables from .env"
	@set -a && . ./.env && set +a && $(MAGIC_BIN)

.PHONY: tidy
tidy:
	$(GO) mod tidy

.PHONY: clean
clean:
	@rm -f $(BINDIR)/*


.PHONY: test
test:
	$(GO) test ./...

.PHONY: run-serve
run-serve:
	swag init --parseDependency --parseDepth 1 -g main.go
	$(GO) run main.go


.PHONY: deploy
deploy:
	@docker-compose --env-file ./.env -f ./docker/local/docker-compose.yml up -d

.PHONY: local
local:
	@cd docker/local && $(MAKE) up

.PHONY: local-down
local-down:
	@cd docker/local && $(MAKE) down

.PHONY: local-stop
local-stop:
	@cd docker/local && $(MAKE) stop

.PHONY: local-reset
local-reset:
	@cd docker/local && $(MAKE) reset

