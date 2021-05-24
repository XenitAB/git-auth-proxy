package token

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"

	"github.com/xenitab/azdo-proxy/pkg/auth"
)

type TokenWriter struct {
	logger logr.Logger
	client kubernetes.Interface
	authz  auth.Authorization
}

const (
	usernameValue = "git"
	usernameKey   = "username"
	passwordKey   = "password"
	tokenKey      = "token"
)

func NewTokenWriter(logger logr.Logger, client kubernetes.Interface, authz auth.Authorization) *TokenWriter {
	return &TokenWriter{
		logger: logger,
		client: client,
		authz:  authz,
	}
}

func (t *TokenWriter) Start(stopCh <-chan struct{}) error {
	t.logger.Info("Starting token writer")

	// create label selector string
	labelSelector := metav1.LabelSelector{MatchLabels: map[string]string{"app.kubernetes.io/managed-by": "azdo-proxy"}}
	labelMap, err := metav1.LabelSelectorAsMap(&labelSelector)
	if err != nil {
		return err
	}
	selectorString := labels.SelectorFromSet(labelMap).String()

	// TODO (Philip): Fix context to list to stopCh
	ctx := context.Background()

	// clean up all secrets managed by azdo-proxy
	oldSecrets, err := t.client.CoreV1().Secrets("").List(ctx, metav1.ListOptions{LabelSelector: selectorString})
	if err != nil {
		return err
	}
	for _, oldSecret := range oldSecrets.Items {
		err := t.client.CoreV1().Secrets(oldSecret.Namespace).Delete(ctx, oldSecret.Name, metav1.DeleteOptions{})
		if err != nil {
			return err
		}
	}

	// initial write of the new secrets
	namespaces := []string{}
	for _, e := range t.authz.GetEndpoints() {
		labels := createSecretLabels(e.Domain, e.Organization, e.Project, e.Repository)
		for _, ns := range e.Namespaces {
			namespaces = append(namespaces, ns)
			t.createSecret(ctx, e.SecretName, ns, e.Token, labels)
		}
	}

	// listen for secrets changes
	informer := cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
				options.LabelSelector = selectorString
				return t.client.CoreV1().Secrets("").List(ctx, options)
			},
			WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
				options.LabelSelector = selectorString
				return t.client.CoreV1().Secrets("").Watch(ctx, options)
			},
		},
		&v1.Secret{}, 0, cache.Indexers{},
	)
	informer.AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			UpdateFunc: t.secretUpdate,
			DeleteFunc: t.secretDelete,
		},
	)
	informer.Run(stopCh)

	t.logger.Info("Token writer stopped")
	return nil
}

func (t *TokenWriter) secretUpdate(oldObj, newObj interface{}) {
	oldSecret := oldObj.(*v1.Secret)
	_, err := t.client.CoreV1().Secrets(oldSecret.Namespace).Update(context.Background(), oldSecret, metav1.UpdateOptions{})
	if err != nil {
		t.logger.Error(err, "could not update modified secret")
		return
	}
}

func (t *TokenWriter) secretDelete(obj interface{}) {
	secret := obj.(*v1.Secret)
	_, org, proj, repo, err := getSecretLabels(*secret)
	if err != nil {
		t.logger.Error(err, "metadata missing in secret labels: %s/%s", secret.Name, secret.Namespace)
		return
	}
	e, err := t.authz.LookupEndpoint(org, proj, repo)
	if err != nil {
		t.logger.Error(err, "deleted secret does not match an endpoint")
		return
	}
	labels := createSecretLabels(e.Domain, e.Organization, e.Project, e.Repository)
	t.createSecret(context.Background(), e.SecretName, secret.Namespace, e.Token, labels)
}

func (t *TokenWriter) createSecret(ctx context.Context, name string, namespace string, token string, labels map[string]string) error {
	secretObject := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    labels,
		},
		StringData: map[string]string{
			usernameKey: usernameValue,
			passwordKey: token,
			tokenKey:    token,
		},
	}
	_, err := t.client.CoreV1().Secrets(namespace).Create(ctx, secretObject, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("Could not create Secret %s in namespace %s: %v", name, namespace, err)
	}
	return nil
}

func createSecretLabels(domain, org, proj, repo string) map[string]string {
	return map[string]string{
		"app.kubernetes.io/managed-by":     "azdo-proxy",
		"azdo-proxy.xenit.io/domain":       domain,
		"azdo-proxy.xenit.io/organization": org,
		"azdo-proxy.xenit.io/project":      proj,
		"azdo-proxy.xenit.io/repository":   repo,
	}
}

func getSecretLabels(secret v1.Secret) (string, string, string, string, error) {
	labels := secret.ObjectMeta.Labels
	keys := []string{"azdo-proxy.xenit.io/domain", "azdo-proxy.xenit.io/organization", "azdo-proxy.xenit.io/project", "azdo-proxy.xenit.io/repository"}
	values := []string{}
	for _, k := range keys {
		v, ok := labels[k]
		if !ok {
			return "", "", "", "", fmt.Errorf("key not found in secret labels: %v", k)
		}
		values = append(values, v)
	}

	return values[0], values[1], values[2], values[3], nil
}
