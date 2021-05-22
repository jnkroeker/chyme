BINARY ?= out/$(NAME)

.PHONY: build
build: NAME=chyme
build:
	go build -o $(BINARY) kroekerlabs.dev/chyme/services/cmd