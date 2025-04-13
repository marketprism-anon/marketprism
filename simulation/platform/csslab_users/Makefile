CWD:=$(shell pwd)
ARCH=amd64 arm64
OS:="linux" "darwin"

all: build

mk-outdir:
	mkdir -p users

build: mk-outdir
	@for i in ${ARCH};do\
		for j in ${OS}; do\
			mkdir -p users/$$j/$$i;\
			echo "    building:" $$i/$$j;\
			GOOS=$$j GOARCH=$$i go build -v -o users/$$j/$$i/users;\
		done;\
	done;

build-rc: mk-outdir
	GOOS=linux GOARCH=amd64 go build -v -race -o users/linux/amd64/users


clean:
	rm -rf users

.PHONY: clean
