FROM docker.io/library/golang:1.20 AS builder

WORKDIR /mockd

COPY . .
# build
RUN go build -o bin/ -tags='netgo timetzdata' -trimpath -a -ldflags '-s -w -linkmode external -extldflags "-static"'  ./cmd/mockd

FROM docker.io/library/alpine:3
LABEL maintainer="The Sia Foundation <info@sia.tech>" \
      org.opencontainers.image.description.vendor="The Sia Foundation" \
      org.opencontainers.image.description="A mock worker API for renterd" \
      org.opencontainers.image.source="https://github.com/SiaFoundation/renterd-mock" \
      org.opencontainers.image.licenses=MIT

ENV PUID=0
ENV PGID=0

# copy binary and prepare data dir.
COPY --from=builder /mockd/bin/* /usr/bin/
VOLUME [ "/data" ]

# API port
EXPOSE 9980/tcp

USER ${PUID}:${PGID}

ENTRYPOINT [ "mockd", "api.addr", ":9980", "dir", "/data" ]