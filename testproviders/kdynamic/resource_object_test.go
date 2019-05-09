package main

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/plugintest"
)

func TestAccResourceObject(t *testing.T) {
	plugintest.Test(t, plugintest.TestCase{
		Providers: testProviders,
		Steps: []plugintest.TestStep{
			{
				Config: testAccResourceObjectConfig(),
				Check: plugintest.ComposeTestCheckFunc(
					plugintest.TestCheckOutput("result_metadata_namespace", "default"),
				),
			},
			{
				Config: testAccResourceObjectConfigUpdated(),
				Check: plugintest.ComposeTestCheckFunc(
					plugintest.TestCheckOutput("result_metadata_namespace", "default"),
				),
			},
		},
	})
}

func testAccResourceObjectConfig() string {
	return `
resource "kdynamic_object" "cmtest" {
	version = "v1"
	kind = "configmaps"

	object = {
		apiVersion = "v1"
		kind = "ConfigMap"
		metadata = {
			name = "cmtest"
		}
		data = {
			key1 = "value1"
			key2 = "value2"
		}
	}
}

output "result_metadata_namespace" {
	value = kdynamic_object.cmtest.result.metadata.namespace
}
`
}

func testAccResourceObjectConfigUpdated() string {
	return `
resource "kdynamic_object" "cmtest" {
	version = "v1"
	kind = "configmaps"

	object = {
		apiVersion = "v1"
		kind = "ConfigMap"
		metadata = {
			name = "cmtest"
		}
		data = {
			key1 = "value1"
			key2 = "value2"
			key3 = 4
		}
	}
}

output "result_metadata_namespace" {
	value = kdynamic_object.cmtest.result.metadata.namespace
}
`
}
