#! /bin/sh
set -e

cleanup() {
  echo "Cleaning Up"
  kill $PID
}

trap cleanup EXIT

# create namespaces
kubectl --dry-run=true -o yaml create namespace azdo-proxy | kubectl apply -f -
kubectl --dry-run=true -o yaml create namespace tenant-1 | kubectl apply -f -
kubectl --dry-run=true -o yaml create namespace tenant-2 | kubectl apply -f -

# install nginx test server
helm repo add bitnami https://charts.bitnami.com/bitnami
helm upgrade --install test bitnami/nginx --namespace azdo-proxy -f ./e2e/nginx-values.yaml

# install azdo-proxy
helm upgrade --install azdo-proxy ./charts/azdo-proxy --namespace azdo-proxy --set image.tag=$1 -f ./e2e/azdo-proxy-values.yaml

# wait for pods to start
kubectl wait --for=condition=available --timeout=600s deployment/test-nginx deployment/azdo-proxy --namespace azdo-proxy

# check that secrets have been created
kubectl -n tenant-1 get secret org-proj-repo
kubectl -n tenant-2 get secret org-proj-repo

# make test http requests
kubectl --namespace azdo-proxy port-forward svc/azdo-proxy 8080:80 &
PID=$!
sleep 2
TOKEN=$(kubectl -n tenant-1 get secret org-proj-repo --template={{.data.token}} | base64 -d -w 0)

STATUS=$(curl -s -o /dev/null -w "%{http_code}" -u username:$TOKEN http://localhost:8080/org/proj/_apis/git/repositories/repo)
if [ $STATUS != "200" ]; then
  exit 1
fi
STATUS=$(curl -s -o /dev/null -w "%{http_code}" -u username:$TOKEN http://localhost:8080/org/proj/_apis/git/repositories/repo1)
if [ $STATUS != "403" ]; then
  exit 1
fi


# All tests are complete
echo "E2E passed"
