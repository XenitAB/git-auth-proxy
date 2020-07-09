TAG = latest
IMG ?= quay.io/xenitab/azdo-proxy:$(TAG)

fmt:
	go fmt ./...

vet:
	go vet ./...

test: fmt vet
	go test ./...

run: fmt vet
	go run main.go

docker-build:
	docker build -t ${IMG} .

kind-load:
	kind load docker-image $(IMG)
