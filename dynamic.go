package sdk

import (
	"encoding/json"

	"github.com/pkg/errors"
	"github.com/zclconf/go-cty/cty"
	ctyjson "github.com/zclconf/go-cty/cty/json"
)

type Dynamic struct {
	Value cty.Value
}

func (d *Dynamic) SetValueFromMap(v map[string]interface{}) error {
	jsonVal, err := json.Marshal(v)
	if err != nil {
		errors.Wrapf(err, "unable to marshal value")
	}

	simple := &ctyjson.SimpleJSONValue{}
	err = simple.UnmarshalJSON(jsonVal)
	if err != nil {
		return errors.Wrapf(err, "unable to unmarshal to simple value")
	}

	d.Value = simple.Value
	return nil
}

func (d *Dynamic) ValueToMap() (map[string]interface{}, error) {
	if d == nil || d.Value == cty.NilVal {
		return nil, nil
	}

	jsonVal, err := ctyjson.SimpleJSONValue{
		Value: d.Value,
	}.MarshalJSON()
	if err != nil {
		return nil, errors.Wrapf(err, "unable to marshal value")
	}

	objRaw := map[string]interface{}{}
	err = json.Unmarshal(jsonVal, &objRaw)
	if err != nil {
		return nil, errors.Wrapf(err, "unable to unmarshal value")
	}

	return objRaw, nil
}
