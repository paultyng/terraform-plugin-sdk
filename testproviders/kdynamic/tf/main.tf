resource "kdynamic_object" "cmtest" {
  version = "v1"
  kind    = "configmaps"

  object = {
    apiVersion = "v1"
    kind       = "ConfigMap"
    metadata = {
      name = "tftest"
    }
    data = {
      key1 = "value1"
      key2 = "value2"
      key3 = "value3"
    }
  }
}

output "cm" {
  value = kdynamic_object.cmtest.result
}

output "result_metadata_namespace" {
  value = kdynamic_object.cmtest.result.metadata.namespace
}

output "result_metadata_resource_version" {
  value = kdynamic_object.cmtest.result.metadata.resourceVersion
}