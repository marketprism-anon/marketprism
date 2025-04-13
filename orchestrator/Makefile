CWD:=$(shell pwd)
ARCH=amd64 arm64
OS:="linux" "darwin"

all: build
	
mk-outdir:
	@mkdir -p orchestrator

build: mk-outdir
	@for i in ${ARCH};do\
		for j in ${OS}; do\
			mkdir -p orchestrator/$$j/$$i;\
			echo "    building:" $$i/$$j;\
			GOOS=$$j GOARCH=$$i go build -v -o orchestrator/$$j/$$i/orchestrator;\
		done;\
	done;

build-rc: mk-outdir 
	GOOS=linux GOARCH=amd64 go build -v -race -o orchestrator/linux/amd64/orchestrator

clean:
	rm -rf orchestrator

.PHONY: clean
