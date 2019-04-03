protobuf:
	protoc -I=. -I=$$GOPATH/src -I=$$GOPATH/src/github.com/gogo/protobuf/protobuf --gogo_out=plugins=grpc:./. ./tfplugin5/tfplugin5.proto
