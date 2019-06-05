# Terraform Plugin SDK

Experimental, code generation based plugin SDK for Terraform plugin protocol 5.0

## How it Works

### Writing a Data Source

To write a data source, you create a Go struct and add tags to the fields:

```go
type dataHTTP struct {
	provider *provider

	URL            urlAttribute      `tf:"url,required"`
	RequestHeaders map[string]string `tf:"request_headers,optional"`
	Body           string            `tf:"body,computed"`
}
```

Resources should implement the `sdk.Resource` interface:

```go
type DataSource interface {
	Read(context.Context) error

	// generated methods
	Schema() Schema
	UnmarshalState(cty.Value) error
	MarshalState() (cty.Value, error)
}
```

The methods `Schema`, `UnmarshalState`, and `MarshalState` will be generated by code-generation, so no need to implement those manually.

### Writing a Resource

Similar to a data source, writing a resource involves creating a struct:

```go
type resourcePrivateKey struct {
	provider *provider

	Algorithm string `tf:"algorithm,required,forcenew"`
	// TODO: should this be computed? https://gist.github.com/paultyng/58ba209b406a7c7f4aa1c9333285a9a2
	RSABits    int    `tf:"rsa_bits,optional,forcenew"`
	ECDSACurve string `tf:"ecdsa_curve,optional,forcenew"`

	PrivateKeyPEM           string `tf:"private_key_pem,computed"`
	PublicKeyPEM            string `tf:"public_key_pem,computed"`
	PublicKeyOpenSSH        string `tf:"public_key_openssh,computed"`
	PublicKeyFingerprintMD5 string `tf:"public_key_fingerprint_md5,computed"`
}
```

And implementing an interface:

```go
type Resource interface {
	Read(context.Context) error
	Create(context.Context) error
	Delete(context.Context) error

	// generated methods
	Schema() Schema
	UnmarshalState(cty.Value) error
	MarshalState() (cty.Value, error)
}
```

You can optionally implement the `Updater` interface:

```go
type Updater interface {
	Update(context.Context) error
}
```

### Code Generation

To generate the missing methods for your interface implementation, add a `go generate` comment to your resource file:

```go
//go:generate tfplugingen -gen datasource -type dataHTTP -name http
```

The generated output will contain the methods mentioned above, as well as an `init` implementation that registers the resource or data source with the provider.

### Advanced Implementation Details

#### Dynamic Attribute Support

You can use the SDK type `Dynamic` for an attribute to allow for dynamic complex types to be consumed by the plugin.

#### Diagnostics vs Errors

Instead of returning a generic `error` from a method implementation, you can instead return `Diagnostics` which allow you to provide richer error and warning information for the user.

#### Validation

TBD

#### Custom Types / Aliases

Attributes support the use of custom types or aliases. This is especially useful for common parsing or validation behaviors.

Those types can implement their own custom validation with the `Validator` interface:

```go
type Validator interface {
	Validate() error
}
```

### Testing

The `plugintest` package has a contract similar to the testing from v1 of the SDK.

## Examples

* [kydnamic](testproviders/kdynamic) - example of a Kubernetes provider using dynamic typing
* [tls](https://github.com/paultyng/terraform-provider-tls/tree/sdk) - TLS provider re-implemented using this SDK
* [http](https://github.com/paultyng/terraform-provider-http/tree/sdk) - HTTP provider re-implemented using this SDK

## TODO

* [ ] Provide access to prior state, config, planned state, etc. in resource / data source methods (potentially via `Context`)
* [ ] Finish implementation of validation, need to generate a wrapper for whole resource and each attribute
* [ ] Add block support for nested structs
