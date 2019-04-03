package main

import (
	"fmt"

	. "github.com/dave/jennifer/jen"
)

func (g *Generator) generateProvider() error {
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

	for _, n := range []struct {
		localPrefix  string
		exportPrefix string
	}{
		{"resource", "Resource"},
		{"dataSource", "DataSource"},
	} {
		factoryType := fmt.Sprintf("%sFactory", n.localPrefix)
		factoryMap := fmt.Sprintf("%sFactories", n.localPrefix)

		g.Type().Id(factoryType).Func().Params(Op("*").Id(g.typeName)).Add(sdk(n.exportPrefix))
		g.Var().Id(factoryMap).Op("=").Map(String()).Id(factoryType).Values()
		g.Func().Params(Id("p").Op("*").Id(g.typeName)).Id(fmt.Sprintf("%sFactory", n.exportPrefix)).Params(Id("typeName").String()).Add(sdk(n.exportPrefix)).Block(
			If(List(Id("f"), Id("ok")).Op(":=").Id(factoryMap).Index(Id("typeName")), Id("ok")).Block(
				Return(Id("f").Params(Id("p"))),
			),
			Panic(Qual("fmt", "Sprintf").Params(Lit(fmt.Sprintf("%s %%s unexpected", n.localPrefix)), Id("typeName"))),
		)
		g.Func().Params(Id("p").Op("*").Id(g.typeName)).Id(fmt.Sprintf("%sSchemas", n.exportPrefix)).Params().Map(String()).Add(sdk("Schema")).Block(
			Id("m").Op(":=").Map(String()).Add(sdk("Schema")).Values(),
			For(List(Id("n"), Id("f")).Op(":=").Range().Id(factoryMap)).Block(
				Id("r").Op(":=").Id("f").Params(Id("p")),
				Id("m").Index(Id("n")).Op("=").Id("r").Dot("Schema").Params(),
			),
			Return(Id("m")),
		)
	}
	return nil
}
