TAG = latest
IMG ?= quay.io/xenitab/azdo-proxy:$(TAG)

assets:
	draw.io -b 10 -x -f png -p 0 -o assets/architecture.png assets/diagram.drawio
.PHONY: assets

.SILENT:
lint:
	golangci-lint run -E misspell

.SILENT:
gosec:
	gosec ./...

.SILENT:
fmt:
	go fmt ./...

.SILENT:
vet:
	go vet ./...

.SILENT:
test: fmt vet
	go test ./...

.SILENT:
run: fmt vet
	go run main.go

.SILENT:
docker-build:
	docker build -t ${IMG} .

.SILENT:
kind-load:
	kind load docker-image $(IMG)
