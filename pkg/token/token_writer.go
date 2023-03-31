package token

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-logr/logr"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"

	"github.com/xenitab/git-auth-proxy/pkg/auth"
)

const (
	managedByLabelKey   = "app.kubernetes.io/managed-by"
	managedByLabelValue = "git-auth-proxy"
	idLabelKey          = "git-auth-proxy.xenit.io/id"
	usernameValue       = "git"
	usernameKey         = "username"
	passwordKey         = "password"
	tokenKey            = "token"
)

type TokenWriter struct {
	client kubernetes.Interface
	authz  *auth.Authorizer
}

func NewTokenWriter(client kubernetes.Interface, authz *auth.Authorizer) *TokenWriter {
	return &TokenWriter{
		client: client,
		authz:  authz,
	}
}

func (t *TokenWriter) Start(ctx context.Context) error {
	log := logr.FromContextOrDiscard(ctx).WithName("token")
	log.Info("Starting token writer")

	// clean up all secrets managed by git-auth-proxy
	labelSelector := metav1.LabelSelector{MatchLabels: map[string]string{managedByLabelKey: managedByLabelValue}}
	labelMap, err := metav1.LabelSelectorAsMap(&labelSelector)
	if err != nil {
		return err
	}
	selectorString := labels.SelectorFromSet(labelMap).String()
	oldSecrets, err := t.client.CoreV1().Secrets("").List(ctx, metav1.ListOptions{LabelSelector: selectorString})
	if err != nil {
		return fmt.Errorf("could not list secrets: %w", err)
	}
	for i := range oldSecrets.Items {
		if err := t.deleteSecret(ctx, oldSecrets.Items[i].Name, oldSecrets.Items[i].Namespace); err != nil {
			return err
		}
	}

	// initial write of the new secrets
	for _, e := range t.authz.GetEndpoints() {
		for _, ns := range e.Namespaces {
			if err := t.createSecret(ctx, e.SecretName, ns, e.Token, e.ID()); err != nil {
				return fmt.Errorf("could not create initial secrets: %w", err)
			}
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
	_, err = informer.AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			UpdateFunc: t.secretUpdated(ctx),
			DeleteFunc: t.secretDeleted(ctx),
		},
	)
	if err != nil {
		return err
	}
	informer.Run(ctx.Done())

	log.Info("Token writer stopped")
	return nil
}

func (t *TokenWriter) secretUpdated(ctx context.Context) func(oldObj, newObj interface{}) {
	log := logr.FromContextOrDiscard(ctx)
	return func(oldObj, newObj interface{}) {
		oldSecret, ok := oldObj.(*v1.Secret)
		if !ok {
			log.Error(errors.New("could not convert to secret"), "could not get old secret")
			return
		}
		err := t.updateSecret(context.Background(), oldSecret, oldSecret.Namespace)
		if err != nil {
			log.Error(err, "secret update error")
			return
		}
	}
}

func (t *TokenWriter) secretDeleted(ctx context.Context) func(obj interface{}) {
	log := logr.FromContextOrDiscard(ctx)
	return func(obj interface{}) {
		secret, ok := obj.(*v1.Secret)
		if !ok {
			log.Error(errors.New("could not convert to secret"), "could not get deleted secret")
			return
		}
		id, ok := secret.ObjectMeta.Annotations[idLabelKey]
		if !ok {
			log.Error(fmt.Errorf("id label not found"), "metadata missing in secret labels", "name", secret.Name, "namespace", secret.Namespace)
			return
		}
		e, err := t.authz.GetEndpointById(id)
		if err != nil {
			log.Error(err, "deleted secret does not match an endpoint", "name", secret.Name, "namespace", secret.Namespace)
			return
		}
		err = t.createSecret(context.Background(), e.SecretName, secret.Namespace, e.Token, e.ID())
		if err != nil {
			log.Error(err, "Unable to created secret after deletion")
			return
		}
	}
}

func (t *TokenWriter) createSecret(ctx context.Context, name string, namespace string, token string, id string) error {
	log := logr.FromContextOrDiscard(ctx).WithName("token")
	secretObject := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				managedByLabelKey: managedByLabelValue,
			},
			Annotations: map[string]string{
				idLabelKey: id,
			},
		},
		StringData: map[string]string{
			usernameKey: usernameValue,
			passwordKey: token,
			tokenKey:    token,
		},
	}
	_, err := t.client.CoreV1().Secrets(namespace).Create(ctx, secretObject, metav1.CreateOptions{})
	if err != nil {
		log.Error(err, "could not create secret", "name", name, "namespace", namespace)
		return err
	}
	log.Info("created secret", "name", name, "namespace", namespace)
	return nil
}

func (t *TokenWriter) updateSecret(ctx context.Context, secret *v1.Secret, namespace string) error {
	log := logr.FromContextOrDiscard(ctx).WithName("token")
	_, err := t.client.CoreV1().Secrets(namespace).Update(ctx, secret, metav1.UpdateOptions{})
	if err != nil {
		log.Error(err, "could not update secret", "name", secret.Name, "namespace", namespace)
		return err
	}
	log.Info("updated secret", "name", secret.Name, "namespace", namespace)
	return nil
}

func (t *TokenWriter) deleteSecret(ctx context.Context, name string, namespace string) error {
	log := logr.FromContextOrDiscard(ctx).WithName("token")
	err := t.client.CoreV1().Secrets(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil {
		log.Error(err, "could not delete old secret", "name", name, "namespace", namespace)
		return err
	}
	log.Info("deleted secret", "name", name, "namespace", namespace)
	return nil
}
