BINARY ?= out/$(NAME)

.PHONY: build
build: NAME=chyme
build:
	go build -o $(BINARY) kroekerlabs.dev/chyme/services/cmd

# --------------------------------------------
# Building Images

CONVERTER_VERSION := 0.1.4

mov_converter:

	docker build --no-cache \
		-f images/mov/Dockerfile \
		-t jnkroeker/mov_converter:${CONVERTER_VERSION} \
		.
