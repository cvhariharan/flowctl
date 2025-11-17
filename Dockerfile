FROM golang:1.25.3-bookworm as go

FROM node:25-bookworm as builder
COPY --from=go /usr/local/go /usr/local/go
ENV PATH "/usr/local/go/bin:$PATH"
ENV GOPATH /go
ENV GOCACHE /go/cache
WORKDIR /build
COPY . .
RUN make

FROM debian:bookworm-slim
WORKDIR /app
COPY --from=builder /build/flowctl /app/flowctl
RUN apt update && apt install -y tzdata

ENTRYPOINT [ "/app/flowctl" ]
CMD [ "start" ]
