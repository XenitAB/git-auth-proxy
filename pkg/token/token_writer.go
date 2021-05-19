package token

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	v1 "k8s.io/api/core/v1"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"

	"github.com/xenitab/azdo-proxy/pkg/auth"
)

type TokenWriter struct {
	logger logr.Logger
	client *kubernetes.Clientset
	authz  auth.Authorization
}

func NewTokenWriter(logger logr.Logger, client *kubernetes.Clientset, authz auth.Authorization) *TokenWriter {
	return &TokenWriter{
		logger: logger,
		client: client,
		authz:  authz,
	}
}

func (t *TokenWriter) Start(stopCh <-chan struct{}) error {
	t.logger.Info("Starting token writer")

	// TODO (Philip): Fix context to list to stopCh
	ctx := context.Background()

	// clean up all secrets managed by azdo-proxy

	// initial write of the new secrets
	namespaces := []string{}
	for _, e := range t.authz.GetEndpoints() {
		labels := map[string]string{
			"app.kubernetes.io/managed-by": "azdo-proxy",
			"domain":                       e.Domain,
			"organization":                 e.Organization,
			"project":                      e.Project,
			"repository":                   e.Repository,
		}
		for _, ns := range e.Namespaces {
			namespaces = append(namespaces, ns)
			t.createSecret(ctx, e.SecretName, ns, e.Token, labels)
		}
	}

	// listen for secrets changes
	// TODO (Philip): Add label selector for informer
	informerFactory := informers.NewSharedInformerFactory(t.client, time.Second*30)
	secretInformer := informerFactory.Core().V1().Secrets()
	secretInformer.Informer().AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			UpdateFunc: t.secretUpdate,
			DeleteFunc: t.secretDelete,
		},
	)
	informerFactory.Start(stopCh)
	informerFactory.WaitForCacheSync(wait.NeverStop)

	t.logger.Info("Token writer stopped")
	return nil
}

func (t *TokenWriter) secretUpdate(old, new interface{}) {

}

func (t *TokenWriter) secretDelete(obj interface{}) {

}

func (t *TokenWriter) createSecret(ctx context.Context, name string, namespace string, token string, labels map[string]string) error {
	secretObject := &v1.Secret{
		ObjectMeta: metaV1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    labels,
		},
		StringData: map[string]string{
			"username": "git",
			"password": token,
			"token":    token,
		},
	}
	_, err := t.client.CoreV1().Secrets(namespace).Create(ctx, secretObject, metaV1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("Could not create Secret %s in namespace %s: %v", name, namespace, err)
	}
	return nil
