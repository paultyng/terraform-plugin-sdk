package main

import (
	"fmt"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"strings"
	"testing"
)

func parseGoType(t *testing.T, src string, lookup string) types.Object {
	t.Helper()

	fset := token.NewFileSet()

	if !strings.HasPrefix(src, "package ") {
		src = fmt.Sprintf("package main\n\n%s", src)
	}

	f, err := parser.ParseFile(fset, "main.go", src, parser.AllErrors|parser.ParseComments)
	if err != nil {
		t.Fatalf("error parsing code: %s", err)
	}

	conf := types.Config{Importer: importer.Default()}

	pkg, err := conf.Check("cmd/hello", fset, []*ast.File{f}, nil)
	if err != nil {
		t.Fatalf("error type checking package: %s", err)
	}

	return pkg.Scope().Lookup(lookup)
}
