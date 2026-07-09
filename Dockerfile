# Web assets are platform-independent: build once on the native builder arch
# (never under QEMU emulation) and reuse for every target platform.
FROM --platform=$BUILDPLATFORM node:20-alpine AS web-build
WORKDIR /web
COPY web/package.json web/package-lock.json* ./
RUN npm ci || npm install
COPY web/ ./
RUN npm run build

# Run the Go toolchain natively on the builder arch and cross-compile to the
# target arch (CGO disabled, so no C toolchain or emulation needed). This keeps
# the slow steps — npm build, sqlc, go build — off QEMU entirely.
FROM --platform=$BUILDPLATFORM golang:1.25-alpine AS go-build
WORKDIR /src
RUN apk add --no-cache git build-base
COPY go.mod go.sum* ./
RUN go mod download
COPY . .

# sqlc output (internal/db/generated) is gitignored — generate it at build time
# so the image builds from a clean checkout without any pre-generated files.
RUN go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.30.0 generate -f internal/db/sqlc.yaml

COPY --from=web-build /web/build ./internal/ui/dist
ARG TARGETOS TARGETARCH
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -ldflags="-s -w" -o /guetteur ./cmd/guetteur

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
COPY --from=go-build /guetteur /usr/local/bin/guetteur
VOLUME ["/data", "/media"]
ENTRYPOINT ["/usr/local/bin/guetteur"]
