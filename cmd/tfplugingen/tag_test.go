package main

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseTag(t *testing.T) {
	for i, c := range []struct {
		expected TagInfo
		tag      string
	}{
		{TagInfo{Name: "url", Required: true}, `tf:"url,required"`},
		{TagInfo{Name: "request_headers", Optional: true}, `tf:"request_headers,optional"`},
		{TagInfo{Name: "body", Computed: true}, `tf:"body,computed"`},
		{TagInfo{Name: "foo", Optional: true, Computed: true}, `tf:"foo,optional,computed"`},
		{TagInfo{Name: "", Required: true}, `tf:",required"`},

		{TagInfo{Omit: true}, `json:"url,omitempty"`},
		{TagInfo{Omit: true}, `tf:"-"`},
	} {
		t.Run(fmt.Sprintf("%d %s", i, c.tag), func(t *testing.T) {
			actual, err := parseTag(c.tag)
			assert.NoError(t, err)
			assert.Equal(t, c.expected, actual)
		})
	}
}
