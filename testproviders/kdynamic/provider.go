package main // import "github.com/hashicorp/terraform-plugin-sdk/testproviders/kubernetes"

import (
	"context"

	sdk "github.com/hashicorp/terraform-plugin-sdk"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/clientcmd"
)

func main() {
	sdk.ServeProvider(New())
}

//go:generate tfplugingen -gen provider -type provider
type provider struct {
}

func New() sdk.Provider {
	return &provider{}
}

func (p *provider) Configure(ctx context.Context, tfVersion string) error {
	// nothing to do here
	return nil
}

func (p *provider) Stop(ctx context.Context) error {
	// nothing to do here
	return nil
}

type restMapper interface {
	RESTMapping(gk schema.GroupKind, versions ...string) (*meta.RESTMapping, error)
	ResourceFor(resource schema.GroupVersionResource) (schema.GroupVersionResource, error)
}

func (p *provider) client() (dynamic.Interface, restMapper, error) {

	config, err := clientcmd.BuildConfigFromFlags("", "/home/paul/.kube/config")
	if err != nil {
		return nil, nil, errors.WithStack(err)
	}

	discoClient, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		return nil, nil, errors.WithStack(err)
	}

	cachedDisco := memory.NewMemCacheClient(discoClient)
	mapper := restmapper.NewDeferredDiscoveryRESTMapper(cachedDisco)

	client, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, nil, errors.WithStack(err)
	}

	return client, mapper, nil
}
