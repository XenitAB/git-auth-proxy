TAG = $$(git rev-parse --short HEAD)
IMG ?= ghcr.io/xenitab/azdo-proxy:$(TAG)

assets:
	draw.io -b 10 -x -f png -p 0 -o assets/architecture.png assets/diagram.drawio
.PHONY: assets

lint:
	golangci-lint run -E misspell

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

e2e:
	# create namespaces
	kubectl --dry-run=true -o yaml create namespace azdo-proxy | kubectl apply -f -
	# install nginx test server
	helm repo add bitnami https://charts.bitnami.com/bitnami
	helm upgrade --install test bitnami/nginx --namespace azdo-proxy
	# install
	helm upgrade --install azdo-proxy ./charts/azdo-proxy --namespace azdo-proxy --set image.tag=$(TAG) -f ./e2e/azdo-proxy-e2e-values.yaml
	# wait for pods to start
	kubectl wait --for=condition=available --timeout=600s deployment/test-nginx deployment/azdo-proxy --namespace azdo-proxy
	# make test http requests
	# clean up namespace
	#kubectl delete namespace azdo-proxy
.PHONY: e2e


