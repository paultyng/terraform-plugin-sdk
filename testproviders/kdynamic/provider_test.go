package main

import (
	sdk "github.com/hashicorp/terraform-plugin-sdk"
)

var testProviders = map[string]sdk.Provider{
	"kdynamic": New(),
}
