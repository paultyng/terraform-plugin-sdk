package plugintest

import (
	"encoding/json"
	"fmt"
	"strings"

	multierror "github.com/hashicorp/go-multierror"
	"github.com/hashicorp/terraform/addrs"
	"github.com/hashicorp/terraform/states"
	"github.com/zclconf/go-cty/cty/gocty"
)

// ComposeTestCheckFunc lets you compose multiple TestCheckFuncs into
// a single TestCheckFunc.
//
// As a user testing their provider, this lets you decompose your checks
// into smaller pieces more easily.
func ComposeTestCheckFunc(fs ...TestCheckFunc) TestCheckFunc {
	return func(s *states.State) error {
		for i, f := range fs {
			if err := f(s); err != nil {
				return fmt.Errorf("Check %d/%d error: %s", i+1, len(fs), err)
			}
		}

		return nil
	}
}

// ComposeAggregateTestCheckFunc lets you compose multiple TestCheckFuncs into
// a single TestCheckFunc.
//
// As a user testing their provider, this lets you decompose your checks
// into smaller pieces more easily.
//
// Unlike ComposeTestCheckFunc, ComposeAggergateTestCheckFunc runs _all_ of the
// TestCheckFuncs and aggregates failures.
func ComposeAggregateTestCheckFunc(fs ...TestCheckFunc) TestCheckFunc {
	return func(s *states.State) error {
		var result *multierror.Error

		for i, f := range fs {
			if err := f(s); err != nil {
				result = multierror.Append(result, fmt.Errorf("Check %d/%d error: %s", i+1, len(fs), err))
			}
		}

		return result.ErrorOrNil()
	}
}

func testCheckResourceAttr(is *states.ResourceInstance, name string, key string, value string) error {
	attrsFlat := is.Current.AttrsFlat
	if attrsFlat == nil {
		err := json.Unmarshal(is.Current.AttrsJSON, &attrsFlat)
		if err != nil {
			return err
		}
	}

	// Empty containers may be elided from the state.
	// If the intent here is to check for an empty container, allow the key to
	// also be non-existent.
	emptyCheck := false
	if value == "0" && (strings.HasSuffix(key, ".#") || strings.HasSuffix(key, ".%")) {
		emptyCheck = true
	}

	if v, ok := attrsFlat[key]; !ok || v != value {
		if emptyCheck && !ok {
			return nil
		}

		if !ok {
			return fmt.Errorf("%s: Attribute '%s' not found", name, key)
		}

		return fmt.Errorf(
			"%s: Attribute '%s' expected %#v, got %#v",
			name,
			key,
			value,
			v)
	}
	return nil
}

// TestCheckResourceAttr is a TestCheckFunc which validates
// the value in state for the given name/key combination.
func TestCheckResourceAttr(name, key, value string) TestCheckFunc {
	return func(s *states.State) error {
		is, err := resourceInstance(s, name)
		if err != nil {
			return err
		}

		return testCheckResourceAttr(is, name, key, value)
	}
}

func resourceInstance(s *states.State, name string) (*states.ResourceInstance, error) {
	abs, diags := addrs.ParseAbsResourceInstanceStr(name)
	if diags.HasErrors() {
		return nil, diags.Err()
	}
	return s.ResourceInstance(abs), nil
}

func testCheckResourceAttrPair(isFirst *states.ResourceInstance, nameFirst string, keyFirst string, isSecond *states.ResourceInstance, nameSecond string, keySecond string) error {
	vFirst, okFirst := isFirst.Current.AttrsFlat[keyFirst]
	vSecond, okSecond := isSecond.Current.AttrsFlat[keySecond]

	// Container count values of 0 should not be relied upon, and not reliably
	// maintained by helper/schema. For the purpose of tests, consider unset and
	// 0 to be equal.
	if len(keyFirst) > 2 && len(keySecond) > 2 && keyFirst[len(keyFirst)-2:] == keySecond[len(keySecond)-2:] &&
		(strings.HasSuffix(keyFirst, ".#") || strings.HasSuffix(keyFirst, ".%")) {
		// they have the same suffix, and it is a collection count key.
		if vFirst == "0" || vFirst == "" {
			okFirst = false
		}
		if vSecond == "0" || vSecond == "" {
			okSecond = false
		}
	}

	if okFirst != okSecond {
		if !okFirst {
			return fmt.Errorf("%s: Attribute %q not set, but %q is set in %s as %q", nameFirst, keyFirst, keySecond, nameSecond, vSecond)
		}
		return fmt.Errorf("%s: Attribute %q is %q, but %q is not set in %s", nameFirst, keyFirst, vFirst, keySecond, nameSecond)
	}
	if !(okFirst || okSecond) {
		// If they both don't exist then they are equally unset, so that's okay.
		return nil
	}

	if vFirst != vSecond {
		return fmt.Errorf(
			"%s: Attribute '%s' expected %#v, got %#v",
			nameFirst,
			keyFirst,
			vSecond,
			vFirst)
	}

	return nil
}

// TestCheckResourceAttrPair is a TestCheckFunc which validates that the values
// in state for a pair of name/key combinations are equal.
func TestCheckResourceAttrPair(nameFirst, keyFirst, nameSecond, keySecond string) TestCheckFunc {
	return func(s *states.State) error {
		isFirst, err := resourceInstance(s, nameFirst)
		if err != nil {
			return err
		}
		isSecond, err := resourceInstance(s, nameSecond)
		if err != nil {
			return err
		}

		return testCheckResourceAttrPair(isFirst, nameFirst, keyFirst, isSecond, nameSecond, keySecond)
	}
}

// TestCheckOutput checks an output in the Terraform configuration
func TestCheckOutput(name, value string) TestCheckFunc {
	return func(s *states.State) error {
		ms := s.RootModule()
		rs, ok := ms.OutputValues[name]
		if !ok {
			return fmt.Errorf("Not found: %s", name)
		}

		expected, err := gocty.ToCtyValue(value, rs.Value.Type())
		if err != nil {
			return err
		}

		if rs.Value.Equals(expected).False() {
			return fmt.Errorf(
				"Output '%s': expected %#v, got %#v",
				name,
				expected,
				rs.Value)
		}

		return nil
	}
}
