ARG GO_IMAGE=golang:1.26-bookworm
FROM ${GO_IMAGE} AS build
WORKDIR /src
COPY go.mod go.sum ./
COPY vendor/ ./vendor/
COPY . .
RUN CGO_ENABLED=0 go build -mod=vendor -trimpath -ldflags="-s -w" -o /out/comms ./cmd/comms \
    && mkdir -p /out/data \
    && chown 65532:65532 /out/data

FROM scratch
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=build /out/comms /comms
COPY --from=build --chown=65532:65532 /out/data/ /data/
USER 65532:65532
WORKDIR /data
ENV COMMS_ADDR=:8080 COMMS_DB_PATH=/data/state.db
EXPOSE 8080
ENTRYPOINT ["/comms"]
