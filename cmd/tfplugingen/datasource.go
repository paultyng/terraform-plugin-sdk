package main

import (
	"github.com/pkg/errors"

	. "github.com/dave/jennifer/jen"
)

func (g *Generator) generateDataSource() error {
	if g.resourceName == "" {
		return errors.Errorf("a resource name is required for type %s", g.typeName)
	}

	err := g.writeSchema()
	if err != nil {
		return err
	}

	err = g.writeUnmarshalState()
	if err != nil {
		return err
	}

	err = g.writeMarshalState()
	if err != nil {
		return err
	}

	g.Func().Id("init").Params().Block(
		Id("dataSourceFactories").Index(Lit(g.resourceName)).Op("=").Func().Params(Id("p").Op("*").Id("provider")).Add(sdk("DataSource")).Block(
			Return(Op("&").Id(g.typeName).Values(Dict{
				Id("provider"): Id("p"),
			})),
		),
	)

	return nil
}
