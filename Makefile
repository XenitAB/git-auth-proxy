TAG = $$(git rev-parse --short HEAD)
IMG ?= ghcr.io/xenitab/git-auth-proxy:$(TAG)

assets:
	draw.io -b 10 -x -f png -p 0 -o assets/architecture.png assets/diagram.drawio
.PHONY: assets

lint:
	golangci-lint run ./...

fmt:
	go fmt ./...

vet:
	go vet ./...

test: fmt vet
	go test --cover ./...

run: fmt vet
	go run main.go

docker-build:
	docker build -t ${IMG} .

kind-load:
	kind load docker-image $(IMG)

e2e: docker-build kind-load
	./e2e/e2e.sh $(TAG)
.PHONY: e2e


