#! /bin/sh
set -xe

cleanup() {
  echo "Cleaning Up"
  kill $PID
}

trap cleanup EXIT

# create namespaces
kubectl --dry-run=client -o yaml create namespace git-auth-proxy | kubectl apply -f -
kubectl --dry-run=client -o yaml create namespace tenant-1 | kubectl apply -f -
kubectl --dry-run=client -o yaml create namespace tenant-2 | kubectl apply -f -

# install nginx test server
helm repo add bitnami https://charts.bitnami.com/bitnami
helm upgrade --install test bitnami/nginx --namespace git-auth-proxy -f ./e2e/nginx-values.yaml

# install git-auth-proxy
helm upgrade --install git-auth-proxy ./charts/git-auth-proxy --namespace git-auth-proxy --set image.tag=$1 -f ./e2e/git-auth-proxy-values.yaml

# wait for pods to start
kubectl wait --for=condition=available --timeout=600s deployment/test-nginx deployment/git-auth-proxy --namespace git-auth-proxy

# check that secret have been created
kubectl -n tenant-1 get secret org-proj-repo
kubectl -n tenant-2 get secret org-proj-repo

# make test http requests
kubectl --namespace git-auth-proxy port-forward svc/git-auth-proxy 8080:80 &
PID=$!
sleep 2
TOKEN=$(kubectl -n tenant-1 get secret org-proj-repo --template={{.data.token}} | base64 -d -w 0)

STATUS=$(curl -s -o /dev/null -w "%{http_code}" -u username:$TOKEN http://localhost:8080/Org/proj/_apis/git/repositories/repo)
if [ $STATUS != "200" ]; then
  exit 1
fi
STATUS=$(curl -s -o /dev/null -w "%{http_code}" -u username:$TOKEN http://localhost:8080/org/proj/_apis/git/repositories/repo1)
if [ $STATUS != "403" ]; then
  exit 1
fi


# All tests are complete
echo "E2E passed"
