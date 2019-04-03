package sdk

import (
	"github.com/pkg/errors"
	"github.com/zclconf/go-cty/cty"
)

func blockType(target interface {
	Schema() Schema
}) cty.Type {
	return target.Schema().Block.impliedType()
}

func unmarshalState(target interface {
	UnmarshalState(cty.Value) error
}, v cty.Value) error {
	if def, ok := target.(Defaulter); ok {
		def.SetDefaults()
	}

	if !v.IsNull() {
		err := target.UnmarshalState(v)
		if err != nil {
			return errors.WithStack(err)
		}
	}

	return nil
}
