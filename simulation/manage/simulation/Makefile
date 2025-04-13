CWD:=$(shell pwd)
ARCH=amd64 arm64
OS:="linux" "darwin"

all: build
	
mk-outdir:
	@mkdir -p simul

mk-simul-files:
	go test -platform=ResearchLab -timeout 9999s

build: mk-outdir mk-simul-files
	@for i in ${ARCH};do\
		for j in ${OS}; do\
			mkdir -p users/$$j/$$i;\
			echo "    building:" $$i/$$j;\
			GOOS=$$j GOARCH=$$i go build -gcflags='all=-N -l' -v -o simul/$$j/$$i/simul;\
		done;\
	done;


build-rc: mk-outdir mk-simul-files
	GOOS=linux GOARCH=amd64 go build -gcflags=all='-N' -v -race -o simul/linux/amd64/simul

clean:
	rm -rf users

.PHONY: clean mk-simul-files
