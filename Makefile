NAS_HOST ?= nas
IMAGE     := guetteur:latest

.PHONY: build build-web test generate clean image deploy

build: build-web
	go build -o bin/guetteur ./cmd/guetteur

build-web:
	cd web && npm run build
	rm -rf internal/ui/dist
	cp -r web/build internal/ui/dist

test:
	go test ./...

generate:
	go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.30.0 generate -f internal/db/sqlc.yaml
	go generate ./...

image:
	docker build --platform linux/arm64 -t $(IMAGE) .

deploy: image
	docker save $(IMAGE) | ssh $(NAS_HOST) docker load

clean:
	rm -rf bin/ web/build internal/ui/dist
