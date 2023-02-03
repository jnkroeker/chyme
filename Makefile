BINARY ?= out/$(NAME)

.PHONY: build
build: NAME=chyme
build:
	go build -o $(BINARY) kroekerlabs.dev/chyme/services/cmd

# --------------------------------------------
# Building Images

MOV_CONVERTER_VERSION := 0.1.4
MP4_PROCESSOR_VERSION := 0.1.5

mov_converter:

	docker build --no-cache \
		-f images/mov/Dockerfile \
		-t jnkroeker/mov_converter:${MOV_CONVERTER_VERSION} \
		.

mp4_converter:

	docker build --no-cache \
		-f images/mp4/Dockerfile \
		-t jnkroeker/mp4_processor:${MP4_PROCESSOR_VERSION} \
		.
