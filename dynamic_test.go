package sdk

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/zclconf/go-cty/cty"
)

func TestValueToMap(t *testing.T) {
	for i, c := range []struct {
		expected map[string]interface{}
		val      cty.Value
	}{
		{nil, cty.NilVal},
		{nil, cty.NullVal(cty.Map(cty.String))},
	} {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			dyn := Dynamic{
				Value: c.val,
			}
			actual, err := dyn.ValueToMap()
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(c.expected, actual) {
				t.Fatal("actual does not match expected")
			}
		})
	}
}
