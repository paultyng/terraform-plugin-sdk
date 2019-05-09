module github.com/hashicorp/terraform-plugin-sdk/testproviders/kubernetes

require (
	github.com/hashicorp/terraform-plugin-sdk v1.0.0
	github.com/imdario/mergo v0.3.7 // indirect
	github.com/pkg/errors v0.8.1
	github.com/zclconf/go-cty v0.0.0-20190430221426-d36a6f0dbffd
	k8s.io/apimachinery v0.0.0-20190507055340-c4317b9d6635
	k8s.io/client-go v0.0.0-20190501104856-ef81ee0960bf
	k8s.io/utils v0.0.0-20190308190857-21c4ce38f2a7 // indirect
)

replace github.com/hashicorp/terraform-plugin-sdk => ../../
