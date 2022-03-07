# Git Auth Proxy

[![Go Report Card](https://goreportcard.com/badge/github.com/XenitAB/git-auth-proxy)](https://goreportcard.com/report/github.com/XenitAB/git-auth-proxy)

Proxy to allow multi-tenant sharing of GitHub and Azure DevOps credentials in Kubernetes.

Most Git providers offer multiple ways of authenticating when cloning repositories and communicating with their API. These authentication methods are usually tied to a specific user and in the best
case offer the ability to scope the permissions. The lack of organization API keys leads to solutions like GitHubs solution to [create a machine user](https://docs.github.com/en/developers/overview/managing-deploy-keys#machine-users)
that has limited permissions. The need for machine user accounts is especially important for GitOps deployment flows with projects like [Flux](https://docs.github.com/en/developers/overview/managing-deploy-keys#machine-users)
and [ArgoCD](https://github.com/argoproj/argo-cd). These tools need an authentication method that supports accessing multiple repositories, without sharing the global credentials with all users.

<p align="center">
  <img src="./assets/architecture.png">
</p>

Git Auth Proxy attempts to solve this problem by implementing its own authentication and authorization layer in between the client and the Git provider. It works by generating static tokens that are
specific to a Git repository. These tokens are then written to a Kubernetes secret in the Kubernetes namespaces which should have access to the repositories. When a repository is cloned through the
proxy, the token will be checked against the repository cloned, and if valid it will be replaced with the correct credentials. The request will be denied if a token is used to clone any other
repository which is does not have access to.

## How To

The proxy reads its configuration from a JSON file. It contains a list of repositories that can be accessed through the proxy and the Kubernetes namespaces which should receive a Secret.

When using Azure DevOps a [PAT](https://docs.microsoft.com/en-us/azure/devops/organizations/accounts/use-personal-access-tokens-to-authenticate?view=azure-devops&tabs=preview-page) has to be
configured for Git Auth Proxy to append to authorized requests. Note that organization and repository names are matched case-insensitive.

```json
{
  "organizations": [
    {
      "provider": "azuredevops",
      "azuredevops": {
        "pat": "<PAT>"
      },
      "host": "dev.azure.com",
      "name": "xenitab",
      "repositories": [
        {
          "name": "fleet-infra",
          "project": "lab",
          "namespaces": [
            "foo",
            "bar"
          ]
        }
      ]
    }
  ]
}
```

When using GitHub a [GitHub Application](https://docs.github.com/en/developers/apps) has to be created and installed. The PEM key needs to be extracted and passed as a base64 encoded string in the
configuration file. Note that the project field is not required when using GitHub as projects do not exists in GitHub.

```json
{
  "organizations": [
    {
      "provider": "github",
      "github": {
        "appID": 123,
        "installationID": 123,
        "privateKey": "<BASE64>"
      },
      "host": "github.com",
      "name": "xenitab",
      "repositories": [
        {
          "name": "fleet-infra",
          "namespaces": [
            "foo",
            "bar"
          ]
        }
      ]
    }
  ]
}
```

Add the Helm repository and install the chart, be sure to set the config content.

```shell
helm repo add https://xenitab.github.io/git-auth-proxy/
helm install git-auth-proxy --set config=<config-json>
```

There should now be a `git-auth-proxy` Deployment and Service in the cluster, ready to proxy traffic.

### Git

Cloning a repository through the proxy is not too different from doing so directly from GitHub or Azure DevOps. The only limitation is that it is not possible to clone through ssh, as Git Auth Proxy
only proxies HTTP(S) traffic. To clone the repository `repo-1` [get the clone URL from the repository page](https://docs.microsoft.com/en-us/azure/devops/repos/git/clone?view=azure-devops&tabs=visual-studio#get-the-clone-url-to-your-repo).
Then replace the host part of the URL with `git-auth-proxy` and add the token as a basic auth parameter. The result should be similar to below.

```shell
git clone http://<token-1>@git-auth-proxy/org/proj/_git/repo-1
```

### API

API calls can also be done through the proxy. Currently only repository specific requests will be permitted as authorization is done per repository. This may change in future releases.

#### GitHub

The proxy assumes that the requests sent to it are in a GitHub enterprise format due to the way GitHub clients behave when configured with a host that is not `github.com`. The main difference between
GitHub Enterprise and non GitHub Enterprise is the API format. The GitHub Enterprise API expects all requests to the API to have the prefix `/api/v3/` while non GitHub Enterprise API requests are sent
to the host `api.github.com`.

#### Azure DevOps

Execute the following command to list all pull requests in the repository `repo-1` using the local token to authenticate to the proxy.

```shell
curl https://<token-1>@git-auth-proxy/org/proj/_apis/git/repositories/repo-1/pullrequests?api-version=5.1
```

> :warning: **If you intend on using a language specific API**: Please read this!

Some APIs built by Microsoft, like [azure-devops-go-api](https://github.com/microsoft/azure-devops-go-api), will make a request to the [Resource Areas API](https://docs.microsoft.com/en-us/azure/devops/extend/develop/work-with-urls?view=azure-devops&tabs=http#how-to-get-an-organizations-url)
which returns a list of location URLs for a specific organization. They will then use those URLs when making additional requests, skipping the proxy. To avoid this you need to explicitly create your
client instead of allowing it to be created automatically.

In the case of Go you should create a client in the following way.

```go
package main

import (
  "github.com/microsoft/azure-devops-go-api/azuredevops"
  "github.com/microsoft/azure-devops-go-api/azuredevops/git"
)

func main() {
  connection := azuredevops.NewAnonymousConnection("http://git-auth-proxy")
  client := connection.GetClientByUrl("http://git-auth-proxy")
  gitClient := &git.ClientImpl{
    Client: *client,
  }
}
```

Instead of the cleaner solution which would ignore the proxy.

```go
package main

import (
  "context"

  "github.com/microsoft/azure-devops-go-api/azuredevops"
  "github.com/microsoft/azure-devops-go-api/azuredevops/git"
)

func main() {
  connection := azuredevops.NewAnonymousConnection("http://git-auth-proxy")
  ctx := context.Background()
  gitClient, _ := git.NewClient(ctx, connection)
}
```

# License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

