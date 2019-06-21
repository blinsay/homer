NAME := homer
PKG := github.com/blinsay/homer

VERSION := $(shell cat VERSION.txt)
GITCOMMIT := $(shell git rev-parse --short HEAD)
GITDIRY := $(shell git status --porcelain --untracked-files=no)
ifneq ($(GITDIRY),)
	GITCOMMIT := $(GITCOMMIT)-dirty
endif

GOOSARCHES := $(shell cat .goosarch)
VERSION_FLAGS=-X $(PKG)/version.VERSION=$(VERSION) -X $(PKG)/version.GITCOMMIT=$(GITCOMMIT)
GO_LDFLAGS=-ldflags "$(VERSION_FLAGS)"
GO_LDFLAGS_STATIC=-ldflags "$(VERSION_FLAGS) -extldflags -static"

.PHONY: all
all: clean build fmt lint test staticcheck install

# build and install

.PHONY: clean
clean:
	@echo "+$@"
	@$(RM) $(NAME)
	@$(RM) -r build/

.PHONY: tag
tag:
	git tag -a $(VERSION) -m $(VERSION)

.PHONY: build
build: $(NAME)

$(NAME): $(wildcard *.go)
	@echo "+$@"
	@go build $(GO_LDFLAGS) -o $(NAME) .

.PHONY: install
install:
	@echo "+$@"
	@go install -a $(GO_LDFLAGS) .

define build_cross
mkdir -p build/;
GOOS=$(1) GOARCH=$(2) CGO_ENABLED=0 go build $(GO_LDFLAGS_STATIC) -o build/$(NAME)-$(1)-$(2) .;
endef

.PHONY: cross
cross:
	@echo "+$@"
	@$(foreach GOOSARCH, $(GOOSARCHES), echo ++$(GOOSARCH) && $(call build_cross,$(subst /,,$(dir $(GOOSARCH))),$(notdir $(GOOSARCH))))

# deps

.PHONY: dep
dep:
	@echo "+$@"
	@dep ensure

# tests

.PHONY: test
	@echo "+$@"
	@go test ./...

# linting and static analysis

.PHONY: fmt
fmt:
	@echo "+$@"
	@gofmt -s -l .

.PHONY: lint
lint:
	@echo "+$@"
	@golint ./... | grep -v vendor | tee /dev/stderr


.PHONY: vet
vet:
	@echo "+$@"
	@go vet ./... | grep -v vendor | tee /dev/stderr

.PHONY: staticcheck
staticcheck:
	@echo "+$@"
	@staticcheck ./...

