FROM golang:1.3
MAINTAINER Jason Wilder <jwilder@litl.com>

COPY . /go/src/github/jwilder/docker-gen
WORKDIR /go/src/github/jwilder/docker-gen

RUN make get-deps
RUN CGO_ENABLED=0 go install -v -a -tags netgo -ldflags "-w -X main.buildVersion \"$(git describe --tags)\"" .

ENV DOCKER_HOST unix:///tmp/docker.sock

ENTRYPOINT ["docker-gen"]
