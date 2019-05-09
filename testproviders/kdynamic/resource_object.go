package main

import (
	"context"
	"fmt"

	sdk "github.com/hashicorp/terraform-plugin-sdk"
	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

const defaultNamespace = "default"

//go:generate tfplugingen -gen resource -type resourceObject -name kdynamic_object
type resourceObject struct {
	provider *provider

	Object sdk.Dynamic `tf:"object,required"`
	Result sdk.Dynamic `tf:"result,computed"`
}

func (r *resourceObject) Create(ctx context.Context) error {
	// see https://github.com/kubernetes/kubernetes/pull/76513/files

	client, mapper, err := r.provider.client()
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

	gvk := obj.GroupVersionKind()

	mapping, err := mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return errors.WithStack(err)
	}

	result, err := client.Resource(mapping.Resource).
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
	client, mapper, err := r.provider.client()
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

	gvk := obj.GroupVersionKind()

	mapping, err := mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return errors.WithStack(err)
	}

	name := obj.GetName()

	result, err := client.Resource(mapping.Resource).
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
	client, mapper, err := r.provider.client()
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

	gvk := obj.GroupVersionKind()

	mapping, err := mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return errors.WithStack(err)
	}

	name := obj.GetName()

	err = client.Resource(mapping.Resource).
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
