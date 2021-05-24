# create namespaces
kubectl --dry-run=true -o yaml create namespace azdo-proxy | kubectl apply -f -
kubectl --dry-run=true -o yaml create namespace tenant-1 | kubectl apply -f -
kubectl --dry-run=true -o yaml create namespace tenant-2 | kubectl apply -f -
# install nginx test server
helm repo add bitnami https://charts.bitnami.com/bitnami
helm upgrade --install test bitnami/nginx --namespace azdo-proxy
# install
helm upgrade --install azdo-proxy ./charts/azdo-proxy --namespace azdo-proxy --set image.tag=$1 -f ./e2e/azdo-proxy-e2e-values.yaml
# wait for pods to start
kubectl wait --for=condition=available --timeout=600s deployment/test-nginx deployment/azdo-proxy --namespace azdo-proxy
# make test http requests
# clean up namespace
#kubectl delete namespace azdo-proxy
