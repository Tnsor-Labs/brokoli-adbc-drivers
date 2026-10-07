# The catalog load check, in an environment shaped like Brokoli's native
# worker image (brokoli-ee Dockerfile.native): Debian trixie, glibc, and the
# same runtime packages. A driver that needs a shared library this
# environment lacks fails here, as it would on a worker.
#
# Keep the apt line in step with Dockerfile.native's runtime stage.
FROM golang:1.26-trixie AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY main.go ./
RUN CGO_ENABLED=1 go build -o /loadcheck .

FROM debian:trixie-slim
RUN apt-get update && \
    apt-get install -y --no-install-recommends ca-certificates git python3 python3-pip tzdata bash && \
    rm -rf /var/lib/apt/lists/*
COPY --from=build /loadcheck /usr/local/bin/loadcheck
ENTRYPOINT ["/usr/local/bin/loadcheck"]
CMD ["/catalog/index.json"]
