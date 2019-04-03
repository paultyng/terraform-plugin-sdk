package sdk // import "github.com/hashicorp/terraform-plugin-sdk"

import (
	"encoding/json"

	pb "github.com/hashicorp/terraform-plugin-sdk/tfplugin5"
	"github.com/pkg/errors"
	"github.com/zclconf/go-cty/cty"
)

func pbDynamicValue(v []byte) *pb.DynamicValue {
	if v == nil {
		return nil
	}
	return &pb.DynamicValue{
		Msgpack: v,
	}
}

func pbSchemaAttribute(v Attribute) (*pb.Schema_Attribute, error) {
	jsonType, err := json.Marshal(v.Type)
	if err != nil {
		return nil, errors.Wrapf(err, "unable to marshal attribute type: %s", v.Name)
	}

	computed := v.Computed
	if v.Optional && v.Type.IsPrimitiveType() {
		// marking optional things as computed here to
		// allow for local defaulting (env vars, static defaults)
		// TODO: figure out how to find just the ones that do default?
		computed = true
	}

	return &pb.Schema_Attribute{
		Name:        v.Name,
		Description: v.Description,
		Type:        jsonType,
		Required:    v.Required,
		Optional:    v.Optional,
		Computed:    computed,
		Sensitive:   v.Sensitive,
	}, nil
}

func pbSchemaBlock(v Block) (*pb.Schema_Block, error) {
	atts := make([]*pb.Schema_Attribute, len(v.Attributes))
	var err error
	for i, att := range v.Attributes {
		atts[i], err = pbSchemaAttribute(att)
		if err != nil {
			return nil, errors.WithStack(err)
		}
	}

	return &pb.Schema_Block{
		Version:    int64(v.Version),
		Attributes: atts,
	}, nil
}

func pbSchema(v Schema) (*pb.Schema, error) {
	block, err := pbSchemaBlock(v.Block)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &pb.Schema{
		Version: int64(v.Version),
		Block:   block,
	}, nil
}

func pbMapSchema(v map[string]Schema) (map[string]*pb.Schema, error) {
	if v == nil {
		return nil, nil
	}

	m := make(map[string]*pb.Schema, len(v))
	var err error
	for k, s := range v {
		m[k], err = pbSchema(s)
		if err != nil {
			return nil, errors.WithStack(err)
		}
	}
	return m, nil
}

func pbAttributePathStep(v cty.PathStep) (*pb.AttributePath_Step, error) {
	switch v := v.(type) {
	case cty.GetAttrStep:
		return &pb.AttributePath_Step{
			Selector: &pb.AttributePath_Step_AttributeName{
				AttributeName: v.Name,
			},
		}, nil
	case cty.IndexStep:
		switch v.Key.Type() {
		case cty.String:
			return &pb.AttributePath_Step{
				Selector: &pb.AttributePath_Step_ElementKeyString{
					ElementKeyString: v.Key.AsString(),
				},
			}, nil
		case cty.Number:
			bf := v.Key.AsBigFloat()
			if !bf.IsInt() {
				return nil, errors.Errorf("key is not an integer: %v", bf)
			}
			i64, _ := bf.Int64()
			return &pb.AttributePath_Step{
				Selector: &pb.AttributePath_Step_ElementKeyInt{
					ElementKeyInt: i64,
				},
			}, nil
		}
		return nil, errors.Errorf("unexpected index value: %#v", v.Key)
	}
	return nil, errors.Errorf("unexpected PathStep type: %T", v)
}

func pbAttributePathSteps(v []cty.PathStep) ([]*pb.AttributePath_Step, error) {
	if v == nil {
		return nil, nil
	}

	steps := make([]*pb.AttributePath_Step, len(v))
	var err error
	for i, s := range v {
		steps[i], err = pbAttributePathStep(s)
		if err != nil {
			return nil, errors.WithStack(err)
		}
	}
	return steps, nil
}

func pbAttributePaths(v []cty.Path) ([]*pb.AttributePath, error) {
	if v == nil {
		return nil, nil
	}

	paths := make([]*pb.AttributePath, len(v))
	for i, p := range v {
		steps, err := pbAttributePathSteps(p)
		if err != nil {
			return nil, errors.WithStack(err)
		}
		paths[i] = &pb.AttributePath{
			Steps: steps,
		}
	}
	return paths, nil
}

func pbDiagnostic(v Diagnostic) (*pb.Diagnostic, error) {
	steps, err := pbAttributePathSteps(v.Path)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &pb.Diagnostic{
		Detail:   v.Detail,
		Summary:  v.Summary,
		Severity: pb.Diagnostic_Severity(v.Severity),
		Attribute: &pb.AttributePath{
			Steps: steps,
		},
	}, nil
}

func pbDiagnostics(v []Diagnostic) ([]*pb.Diagnostic, error) {
	if v == nil {
		return nil, nil
	}

	diags := make([]*pb.Diagnostic, len(v))
	var err error
	for i, d := range v {
		diags[i], err = pbDiagnostic(d)
		if err != nil {
			return nil, errors.WithStack(err)
		}
	}
	return diags, nil
}
