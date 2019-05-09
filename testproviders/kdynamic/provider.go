package main // import "github.com/hashicorp/terraform-plugin-sdk/testproviders/kubernetes"

import (
	"context"

	"github.com/pkg/errors"
	sdk "github.com/hashicorp/terraform-plugin-sdk"
	"k8s.io/client-go/dynamic"
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

func (p *provider) client() (dynamic.Interface, error) {
	config, err := clientcmd.BuildConfigFromFlags("", "/home/paul/.kube/config")
	if err != nil {
		return nil, errors.WithStack(err)
	}

	client, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return client, nil
}