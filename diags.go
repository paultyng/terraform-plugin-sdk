package sdk // import "github.com/hashicorp/terraform-plugin-sdk"

import (
	"strings"

	pb "github.com/hashicorp/terraform-plugin-sdk/tfplugin5"
	"github.com/zclconf/go-cty/cty"
)

type Severity int

const (
	SeverityError   = Severity(pb.Diagnostic_ERROR)
	SeverityWarning = Severity(pb.Diagnostic_WARNING)
)

func errorOrDiagnostics(err error) (Diagnostics, error) {
	diags, ok := err.(Diagnostics)
	if ok {
		return diags, nil
	}
	return nil, err
}

type Diagnostics []Diagnostic

func (diags Diagnostics) Error() string {
	if len(diags) == 0 {
		return "empty diagnostics"
	}

	var sb strings.Builder
	for i, d := range diags {
		if i > 0 {
			sb.WriteString("; ")
		}
		sb.WriteString(d.Summary)
	}
	return sb.String()
}

func (diags Diagnostics) IsError() bool {
	for _, d := range diags {
		if d.Severity == SeverityError {
			return true
		}
	}

	return false
}

type Diagnostic struct {
	Path     cty.Path
	Severity Severity
	Summary  string
	Detail   string
}

func AttributeError(msg string, path cty.Path) Diagnostics {
	// check for any bad steps
	// TODO: maybe just log these?
	// TODO: put stack trace in detail?
	for _, s := range path {
		if s == nil {
			return Diagnostics{
				Diagnostic{
					Severity: SeverityError,
					Summary:  msg,
					Detail:   msg,
				},
				Diagnostic{
					Severity: SeverityWarning,
					Summary:  "Missing attribute path step",
					Detail:   "Missing attribute path step",
				},
			}
		}
	}

	return Diagnostics{
		Diagnostic{
			Path:     path,
			Severity: SeverityError,
			Summary:  msg,
			Detail:   msg,
		},
	}
}
