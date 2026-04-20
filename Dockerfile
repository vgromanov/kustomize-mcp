# syntax=docker/dockerfile:1
FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/kustomize-mcp ./cmd/kustomize-mcp

FROM alpine:3.21
RUN apk add --no-cache ca-certificates
COPY --from=build /out/kustomize-mcp /usr/local/bin/kustomize-mcp
ENTRYPOINT ["/usr/local/bin/kustomize-mcp"]
