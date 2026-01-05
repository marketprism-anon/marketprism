PWD:= $(shell pwd)
build:
	docker build -t race-buildenv -f ubu2004golang.Dockerfile .

run: build
	docker run -it -v $(PWD)/..:/workspace race-buildenv bash

build-rc: build
	docker run -v $(PWD)/..:/workspace race-buildenv make build-rc
