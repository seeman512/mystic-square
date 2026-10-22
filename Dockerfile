FROM golang:1.27.1-alpine AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG TARGETOS=linux
ARG TARGETARCH
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags='-s -w' -o /out/mystic-square ./cmd/api

FROM alpine:3.22

RUN addgroup -S app && adduser -S -G app app \
    && apk add --no-cache ca-certificates

COPY --from=builder /out/mystic-square /usr/local/bin/mystic-square

USER app
WORKDIR /app
EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/mystic-square"]
