package plugintest

import (
	"bytes"
	"fmt"

	"github.com/hashicorp/terraform/command/format"
	"github.com/hashicorp/terraform/tfdiags"
	"github.com/mitchellh/colorstring"
)

// operationError is a specialized implementation of error used to describe
// failures during one of the several operations performed for a particular
// test case.
type operationError struct {
	OpName string
	Diags  tfdiags.Diagnostics
}

func newOperationError(opName string, diags tfdiags.Diagnostics) error {
	return operationError{opName, diags}
}

// Error returns a terse error string containing just the basic diagnostic
// messages, for situations where normal Go error behavior is appropriate.
func (err operationError) Error() string {
	return fmt.Sprintf("errors during %s: %s", err.OpName, err.Diags.Err().Error())
}

// ErrorDetail is like Error except it includes verbosely-rendered diagnostics
// similar to what would come from a normal Terraform run, which include
// additional context not included in Error().
func (err operationError) ErrorDetail() string {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "errors during %s:", err.OpName)
	clr := &colorstring.Colorize{Disable: true, Colors: colorstring.DefaultColors}
	for _, diag := range err.Diags {
		diagStr := format.Diagnostic(diag, nil, clr, 78)
		buf.WriteByte('\n')
		buf.WriteString(diagStr)
	}
	return buf.String()
}

// detailedErrorMessage is a helper for calling ErrorDetail on an error if
// it is an operationError or just taking Error otherwise.
func detailedErrorMessage(err error) string {
	switch tErr := err.(type) {
	case operationError:
		return tErr.ErrorDetail()
	default:
		return err.Error()
	}
}
