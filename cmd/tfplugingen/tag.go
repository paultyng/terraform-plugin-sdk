package main

import (
	"reflect"
	"strings"
)

const (
	tagKey = "tf"

	tagRequired = "required"
	tagOptional = "optional"
	tagComputed = "computed"

	tagForceNew  = "forcenew"
	tagSensitive = "sensitive"
)

type TagInfo struct {
	Name string

	Omit bool

	// Attribute values
	Required  bool
	Optional  bool
	Computed  bool
	ForceNew  bool
	Sensitive bool
}

func stringInSlice(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}

func parseTag(tag string) (TagInfo, error) {
	st := reflect.StructTag(tag)
	tagValue, ok := st.Lookup(tagKey)
	if !ok {
		return TagInfo{Omit: true}, nil
	}
	values := strings.Split(tagValue, ",")
	name, values := values[0], values[1:]

	if name == "-" {
		return TagInfo{Omit: true}, nil
	}

	return TagInfo{
		Name: name,

		Required:  stringInSlice(tagRequired, values),
		Optional:  stringInSlice(tagOptional, values),
		Computed:  stringInSlice(tagComputed, values),
		ForceNew:  stringInSlice(tagForceNew, values),
		Sensitive: stringInSlice(tagSensitive, values),
	}, nil
}
