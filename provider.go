package sdk // import "github.com/hashicorp/terraform-plugin-sdk"

import (
	"context"
	"fmt"

	"github.com/zclconf/go-cty/cty"
)

type Provider interface {
	Configure(context.Context, string) error
	// TODO: make this optional, cast to see if it exists?
	Stop(context.Context) error

	// generated methods
	Schema() Schema
	DataSourceSchemas() map[string]Schema
	ResourceSchemas() map[string]Schema
	DataSourceFactory(string) DataSource
	ResourceFactory(string) Resource
	UnmarshalState(cty.Value) error
	MarshalState() (cty.Value, error)
}

type DataSource interface {
	Read(context.Context) error

	// generated methods
	Schema() Schema
	UnmarshalState(cty.Value) error
	MarshalState() (cty.Value, error)
}

type Resource interface {
	Read(context.Context) error
	Create(context.Context) error
	Delete(context.Context) error

	// generated methods
	Schema() Schema
	UnmarshalState(cty.Value) error
	MarshalState() (cty.Value, error)
}

type Defaulter interface {
	SetDefaults()
}

type Updater interface {
	Update(context.Context) error
}

type Validator interface {
	Validate() error
}

type Schema struct {
	Version int
	Block   Block
}

type Block struct {
	Version    int
	Attributes Attributes
	// TODO: Blocks []NestedBlock
}

func (b Block) ApplyPath(path cty.Path) (*Attribute, error) {
	if len(path) < 1 {
		return nil, fmt.Errorf("path length must be at least 1")
	}

	// attributes are not nested, so just return the first step
	get, ok := path[0].(cty.GetAttrStep)
	if !ok {
		panic("bad first path step")
	}
	return b.Attributes.Lookup(get.Name), nil
}

func (b Block) impliedType() cty.Type {
	atts := map[string]cty.Type{}
	for _, att := range b.Attributes {
		atts[att.Name] = att.Type
	}
	return cty.Object(atts)
}

type Attributes []Attribute

func (atts Attributes) Lookup(name string) *Attribute {
	for _, att := range atts {
		if att.Name == name {
			return &att
		}
	}
	return nil
}

type Attribute struct {
	Name        string
	Description string

	Type cty.Type

	Required bool
	Optional bool
	Computed bool

	Sensitive bool

	ForceNew bool
}

func (att *Attribute) IsArgument() bool {
	if att == nil {
		return false
	}
	return att.Required || att.Optional
}

type doesNotExistError struct{}

func (err *doesNotExistError) Error() string {
	return "resource does not exist"
}

func DoesNotExistError() error {
	return &doesNotExistError{}
}
