FROM golang:1.21.4 as builder

COPY go.mod /src/
COPY go.sum /src/
COPY main.go /src/
WORKDIR /src
RUN go mod download
RUN go build -o device-plugin-example

FROM ubuntu:22.04
WORKDIR /
COPY --from=builder /src/device-plugin-example /device-plugin-example
ENTRYPOINT ["/device-plugin-example"]
