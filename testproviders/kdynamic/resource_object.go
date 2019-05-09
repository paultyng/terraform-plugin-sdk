package main

import (
	"context"
	"fmt"

	sdk "github.com/hashicorp/terraform-plugin-sdk"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

const defaultNamespace = "default"

//go:generate tfplugingen -gen resource -type resourceObject -name kdynamic_object
type resourceObject struct {
	provider *provider

	Group   string `tf:"group,optional,forcenew"`
	Version string `tf:"version,required,forcenew"`
	Kind    string `tf:"kind,required,forcenew"`
	
	Object sdk.Dynamic `tf:"object,required"`
	Result sdk.Dynamic `tf:"result,computed"`
}

func (r *resourceObject) gvr() schema.GroupVersionResource {
	// TODO: figure these out automatically? hopefully can just infer this from the object?
	return schema.GroupVersionResource{
		Group:    r.Group,
		Version:  r.Version,
		Resource: r.Kind,
	}
}

func (r *resourceObject) Create(ctx context.Context) error {
	// see https://github.com/kubernetes/kubernetes/pull/76513/files

	client, err := r.provider.client()
	if err != nil {
		return errors.WithStack(err)
	}

	objRaw, err := r.Object.ValueToMap()
	if err != nil {
		return errors.WithStack(err)
	}

	obj := &unstructured.Unstructured{
		Object: objRaw,
	}

	result, err := client.Resource(r.gvr()).
		Namespace(defaultNamespace).
		Create(obj, metav1.CreateOptions{})
	if err != nil {
		if serr, ok := err.(*apierrors.StatusError); ok {
			fmt.Printf("Err Status: %#v\n", serr.ErrStatus)
			fmt.Printf("Err Details: %#v\n", serr.ErrStatus.Details)
		}
		return errors.WithStack(err)
	}

	r.Result = sdk.Dynamic{}
	r.Result.SetValueFromMap(result.Object)

	return nil
}

func (r *resourceObject) Read(ctx context.Context) error {
	client, err := r.provider.client()
	if err != nil {
		return errors.WithStack(err)
	}

	objRaw, err := r.Object.ValueToMap()
	if err != nil {
		return errors.WithStack(err)
	}

	obj := &unstructured.Unstructured{
		Object: objRaw,
	}

	name := obj.GetName()

	result, err := client.Resource(r.gvr()).
		Namespace(defaultNamespace).
		Get(name, metav1.GetOptions{})
	if err != nil {
		if serr, ok := err.(*apierrors.StatusError); ok {
			fmt.Printf("Err Status: %#v\n", serr.ErrStatus)
			fmt.Printf("Err Details: %#v\n", serr.ErrStatus.Details)
		}
		return errors.WithStack(err)
	}

	r.Result = sdk.Dynamic{}
	r.Result.SetValueFromMap(result.Object)

	return nil	
}

func (r *resourceObject) Delete(ctx context.Context) error {
	client, err := r.provider.client()
	if err != nil {
		return errors.WithStack(err)
	}

	objRaw, err := r.Object.ValueToMap()
	if err != nil {
		return errors.WithStack(err)
	}

	obj := &unstructured.Unstructured{
		Object: objRaw,
	}

	name := obj.GetName()

	err = client.Resource(r.gvr()).
		Namespace(defaultNamespace).
		Delete(name, &metav1.DeleteOptions{})
	if err != nil {
		if serr, ok := err.(*apierrors.StatusError); ok {
			fmt.Printf("Err Status: %#v\n", serr.ErrStatus)
			fmt.Printf("Err Details: %#v\n", serr.ErrStatus.Details)
		}
		return errors.WithStack(err)
	}

	return nil
}
