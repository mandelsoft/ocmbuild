NAME      := ocm-build
PROVIDER  ?= ocm.software

REPO_ROOT                                      := $(shell dirname $(realpath $(lastword $(MAKEFILE_LIST))))/../..


.PHONY: build
build:
	go run . -f
