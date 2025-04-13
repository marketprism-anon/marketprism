FROM ubuntu:20.04

RUN mkdir /workspace
RUN apt-get update && apt-get install -y build-essential ca-certificates wget
RUN wget https://go.dev/dl/go1.19.5.linux-amd64.tar.gz \
   && rm -rf /usr/local/go && tar -C /usr/local -xzf go1.19.5.linux-amd64.tar.gz 

ENV PATH="${PATH}:/usr/local/go/bin"

WORKDIR /workspace
CMD "/bin/bash"
