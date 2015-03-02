FROM scratch

COPY ./docker-gen /docker-gen
COPY ./certs/ca-certificates.crt /etc/ssl/ca-certificates.crt

ENV DOCKER_HOST unix:///tmp/docker.sock

ENTRYPOINT ["/docker-gen"]
