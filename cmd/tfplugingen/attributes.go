package main

import (
	"fmt"
	"go/types"
	"strings"

	. "github.com/dave/jennifer/jen"
	"github.com/pkg/errors"
)

func ifErrReturnErr(returns ...Code) *Statement {
	// TODO: make with stack optional
	returnsWithErr := append(returns, Qual("github.com/pkg/errors", "WithStack").Params(Err()))
	return If(Err().Op("!=").Nil()).Block(Return(returnsWithErr...))
}

func cty(name string) *Statement {
	return Qual("github.com/zclconf/go-cty/cty", name)
}

func gocty(name string) *Statement {
	return Qual("github.com/zclconf/go-cty/cty/gocty", name)
}

func sdk(name string) *Statement {
	return Qual("github.com/hashicorp/terraform-plugin-sdk", name)
}

func isNamedType(t types.Type, path, name string) bool {
	n, ok := t.(*types.Named)
	if !ok {
		return false
	}
	if n.Obj().Name() != name {
		return false
	}
	if n.Obj().Pkg().Path() != path {
		return false
	}
	return true
}

func isTypeSDKDynamic(t types.Type) bool {
	return isNamedType(t, "github.com/hashicorp/terraform-plugin-sdk", "Dynamic")
}

func isTypeTimeTime(t types.Type) bool {
	return isNamedType(t, "time", "Time")
}

func goType(typeExpr types.Type) (*Statement, error) {
	switch t := typeExpr.(type) {
	default:
		return nil, errors.Errorf("unexpected type: %T %#v", typeExpr, typeExpr)
	case *types.Basic:
		return Id(t.Name()), nil
	case *types.Pointer:
		elemType, err := goType(t.Elem())
		if err != nil {
			return nil, errors.WithStack(err)
		}
		return Op("*").Add(elemType), nil
	case *types.Named:
		switch {
		case isTypeTimeTime(t):
			return goType(types.Typ[types.String])
		case !t.Obj().Exported():
			// TODO: detect if the pkg is the current pkg, probably
			// need to add a receiver to this method for more info
			return Id(t.Obj().Name()), nil
		}
		return Qual(t.Obj().Pkg().Path(), t.Obj().Name()), nil
	case *types.Slice:
		elemType, err := goType(t.Elem())
		if err != nil {
			return nil, errors.WithStack(err)
		}
		return Index().Add(elemType), nil
	}
}

func ctyType(typeExpr types.Type) (Code, error) {
	switch t := typeExpr.(type) {
	default:
		return nil, errors.Errorf("unexpected type expression: %T %#v", t, t)
	case *types.Pointer:
		return ctyType(t.Elem())
	case *types.Basic:
		switch t.Kind() {
		case types.String:
			return cty("String"), nil
		case types.Bool:
			return cty("Bool"), nil
		case types.Int, types.Int8, types.Int16, types.Int32,
			types.Int64, types.Uint, types.Uint8, types.Uint16,
			types.Uint32, types.Uint64, types.Uintptr, types.Float32,
			types.Float64:
			return cty("Number"), nil
		default:
			return nil, errors.Errorf("unexpected basic kind: %v", t.Kind())
		}
	case *types.Named:
		switch {
		case isTypeSDKDynamic(t):
			return cty("DynamicPseudoType"), nil
		case isTypeTimeTime(t):
			return ctyType(types.Typ[types.String])
		}
		return ctyType(t.Underlying())
	case *types.Struct:
		structFields := Dict{}
		err := eachAttribute(t, func(tag TagInfo, field *types.Var, embeds []*types.Var) error {
			fieldType, err := ctyType(field.Type())
			if err != nil {
				return errors.Wrapf(err, "error finding type for struct field %s", field.Name())
			}
			structFields[Lit(tag.Name)] = fieldType
			return nil
		})
		if err != nil {
			return nil, errors.WithStack(err)
		}
		return cty("Object").Params(Map(String()).Add(cty("Type")).Values(structFields)), nil
	case *types.Slice:
		elem, err := ctyType(t.Elem())
		if err != nil {
			return nil, errors.WithStack(err)
		}
		return cty("List").Params(elem), nil
	case *types.Map:
		if b, ok := t.Key().(*types.Basic); !ok || b.Kind() != types.String {
			return nil, errors.Errorf("map must have string key, got %T %#v", t.Key(), t.Key())
		}
		elem, err := ctyType(t.Elem())
		if err != nil {
			return nil, errors.WithStack(err)
		}
		return cty("Map").Params(elem), nil
	}
}

func goctyAssignToCty(source, target *Statement, attributeType *types.Named, assignType types.Type) ([]Code, error) {
	targetCty, err := ctyType(assignType)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return []Code{Block(
		List(target.Clone(), Err()).Op("=").Add(gocty("ToCtyValue")).Params(source.Clone(), targetCty),
		ifErrReturnErr(cty("NilVal")),
	)}, nil
}

func assignToCty(source, target *Statement, attributeType *types.Named, assignType types.Type, depth int) ([]Code, error) {
	switch t := assignType.(type) {
	default:
		return nil, errors.Errorf("unexpected type expression: %T %#v", t, t)
	case *types.Struct:
		stateVar := fmt.Sprintf("state%d", depth)
		stmts := []Code{
			Id(stateVar).Op(":=").Map(String()).Add(cty("Value")).Values(),
		}
		err := eachAttribute(t, func(tag TagInfo, field *types.Var, embeds []*types.Var) error {
			fieldTarget := Id(stateVar).Index(Lit(tag.Name))
			fieldSource := source.Clone().Dot(field.Name())
			assign, err := assignToCty(fieldSource.Clone(), fieldTarget.Clone(), nil, field.Type(), depth+1)
			if err != nil {
				return errors.Wrapf(err, "error building assignment for field %s", field.Name())
			}
			stmts = append(stmts, assign...)
			return nil
		})
		if err != nil {
			return nil, errors.WithStack(err)
		}
		stmts = append(stmts, target.Clone().Op("=").Add(cty("ObjectVal").Params(Id(stateVar))))
		return []Code{Block(stmts...)}, nil
	case *types.Pointer:
		return assignToCty(source.Clone(), target.Clone(), attributeType, t.Elem(), depth)
	case *types.Basic:
		switch t.Kind() {
		case types.Bool, types.String, types.Int, types.Int8, types.Int16,
			types.Int32, types.Int64, types.Uint, types.Uint8, types.Uint16,
			types.Uint32, types.Uint64, types.Uintptr:
			return goctyAssignToCty(source.Clone(), target.Clone(), attributeType, t)
		default:
			return nil, errors.Errorf("unexpected basic kind: %v", t.Kind())
		}
	case *types.Named:
		switch {
		case isTypeSDKDynamic(t):
			return []Code{
				target.Clone().Op("=").Add(source.Clone()).Dot("Value"),
			}, nil
		case isTypeTimeTime(t):
			source = source.Clone().Dot("Format").Params(Qual("time", "RFC3339"))
			return goctyAssignToCty(source.Clone(), target.Clone(), attributeType, types.Typ[types.String])
		}
		return assignToCty(source.Clone(), target.Clone(), t, t.Underlying(), depth)
	case *types.Slice:
		return goctyAssignToCty(source.Clone(), target.Clone(), attributeType, t)
	case *types.Map:
		return goctyAssignToCty(source.Clone(), target.Clone(), attributeType, t)
	}
}

func goctyAssignFromCty(source, target *Statement, assignType types.Type) ([]Code, error) {
	// TODO: handle assigning zero values to reset fields?
	ifAssign := If(Op("!").Add(source.Clone()).Dot("IsNull").Params().Op("&&").Add(source.Clone()).Dot("IsKnown").Params())
	return []Code{ifAssign.Block(
		Err().Op("=").Add(gocty("FromCtyValue")).Params(source, Op("&").Add(target.Clone())),
		ifErrReturnErr(),
	)}, nil
}

func assignFromCty(source, target *Statement, attributeType types.Type, assignType types.Type) ([]Code, error) {
	switch t := assignType.(type) {
	default:
		return nil, errors.Errorf("unexpected type expression: %T %#v", t, t)
	case *types.Pointer:
		if attributeType == nil {
			attributeType = t
		}
		return assignFromCty(source.Clone(), target.Clone(), attributeType, t.Elem())
	case *types.Struct:
		stmts := []Code{}
		err := eachAttribute(t, func(tag TagInfo, field *types.Var, embeds []*types.Var) error {
			fieldTarget := target.Clone().Dot(field.Name())
			fieldSource := source.Clone().Dot("GetAttr").Params(Lit(tag.Name))
			assign, err := assignFromCty(fieldSource.Clone(), fieldTarget.Clone(), nil, field.Type())
			if err != nil {
				return errors.Wrapf(err, "error building assignment for field %s", field.Name())
			}
			stmts = append(stmts, assign...)
			return nil
		})
		if err != nil {
			return nil, errors.WithStack(err)
		}
		return []Code{If(Op("!").Add(source.Clone()).Dot("IsNull").Params().Op("&&").Add(source.Clone()).Dot("IsKnown").Params()).Block(stmts...)}, nil
	case *types.Basic:
		switch t.Kind() {
		case types.Bool, types.String, types.Int, types.Int8, types.Int16,
			types.Int32, types.Int64, types.Uint, types.Uint8, types.Uint16,
			types.Uint32, types.Uint64, types.Uintptr:
			if attributeType == nil {
				attributeType = assignType
			}
			return goctyAssignFromCty(source.Clone(), target.Clone(), attributeType)
		default:
			return nil, errors.Errorf("unexpected basic kind: %v", t.Kind())
		}
	case *types.Named:
		switch {
		case isTypeSDKDynamic(t):
			return []Code{
				target.Clone().Op("=").Add(sdk("Dynamic")).Values(Dict{
					Id("Value"): source.Clone(),
				}),
			}, nil
		case isTypeTimeTime(t):
			ifAssign := If(Op("!").Add(source.Clone()).Dot("IsNull").Params().Op("&&").Add(source.Clone()).Dot("IsKnown").Params())
			return []Code{ifAssign.Block(
				List(target.Clone(), Err()).Op("=").Qual("time", "Parse").Params(Qual("time", "RFC3339"), source.Clone().Dot("AsString").Params()),
				ifErrReturnErr(),
			)}, nil
		}
		return assignFromCty(source.Clone(), target.Clone(), t, t.Underlying())
	case *types.Slice, *types.Map:
		if attributeType == nil {
			attributeType = assignType
		}
		return goctyAssignFromCty(source.Clone(), target.Clone(), attributeType)
	}
}

func eachAttribute(st *types.Struct, cb func(TagInfo, *types.Var, []*types.Var) error) error {
	var eachInternal func(embeds []*types.Var, st *types.Struct, cb func(TagInfo, *types.Var, []*types.Var) error) error
	eachInternal = func(embeds []*types.Var, st *types.Struct, cb func(TagInfo, *types.Var, []*types.Var) error) error {
		for i := 0; i < st.NumFields(); i++ {
			field := st.Field(i)

			if field.Embedded() {
				embeds := append(embeds, field)
				// TODO: do this more carefully...
				embedSt := field.Type().(*types.Named).Underlying().(*types.Struct)
				err := eachInternal(embeds, embedSt, cb)
				if err != nil {
					return errors.WithStack(err)
				}
			}

			if !field.Exported() {
				// field is unexported; skipping
				continue
			}

			tag := st.Tag(i)
			tagOpts, err := parseTag(tag)
			if err != nil {
				return errors.WithStack(err)
			}

			if tagOpts.Omit {
				continue
			}

			// TODO: move these checks to the SDK itself?
			for _, rule := range []struct {
				errorf string
				check  func(opts TagInfo) bool
			}{
				{"attributes cannot be both required and optional: %s", func(tagOpts TagInfo) bool { return tagOpts.Required && tagOpts.Optional }},
				{"attributes cannot be both required and computed: %s", func(tagOpts TagInfo) bool { return tagOpts.Required && tagOpts.Computed }},
				{"attributes must be required, optional, or computed: %s", func(tagOpts TagInfo) bool { return !tagOpts.Required && !tagOpts.Optional && !tagOpts.Computed }},
				{"force new attributes must be required or optional: %s", func(tagOpts TagInfo) bool { return tagOpts.ForceNew && !tagOpts.Required && !tagOpts.Optional }},
				{"force new attributes cannot be computed: %s", func(tagOpts TagInfo) bool { return tagOpts.ForceNew && tagOpts.Computed }},
			} {
				if invalid := rule.check(tagOpts); invalid {
					return errors.Errorf(rule.errorf, tag)
				}
			}

			if tagOpts.Name == "" {
				// TODO: lower snake case this...
				tagOpts.Name = strings.ToLower(field.Name())
			}

			err = cb(tagOpts, field, embeds)
			if err != nil {
				return errors.WithStack(err)
			}
		}
		return nil
	}
	return eachInternal(nil, st, cb)
}

func (g *Generator) writeSchema() error {
	atts := []Code{}
	err := eachAttribute(g.typesStruct, func(tag TagInfo, field *types.Var, embeds []*types.Var) error {

		ct, err := ctyType(field.Type())
		if err != nil {
			return errors.WithStack(err)
		}

		atts = append(atts, sdk("Attribute").Values(Dict{
			Id("Name"):      Lit(tag.Name),
			Id("Required"):  Lit(tag.Required),
			Id("Optional"):  Lit(tag.Optional),
			Id("Computed"):  Lit(tag.Computed),
			Id("ForceNew"):  Lit(tag.ForceNew),
			Id("Sensitive"): Lit(tag.Sensitive),
			Id("Type"):      ct,
		}))
		return nil
	})
	if err != nil {
		return err
	}

	g.Func().Params(Id("r").Op("*").Id(g.typeName)).Id("Schema").Params().Add(sdk("Schema")).Block(
		Return(sdk("Schema").Values(Dict{
			Id("Block"): sdk("Block").Values(Dict{
				Id("Attributes"): Index().Add(sdk("Attribute")).Values(atts...),
			}),
		})),
	)

	return nil
}

func (g *Generator) writeUnmarshalState() error {
	stmts := []Code{}
	target := Id("r")
	source := Id("conf")
	assign, err := assignFromCty(source.Clone(), target.Clone(), nil, g.typesStruct)
	if err != nil {
		return errors.WithStack(err)
	}

	if len(assign) > 0 {
		stmts = append([]Code{
			Var().Err().Error(),
			Id("_").Op("=").Err(),
		}, assign...)
	}
	stmts = append(stmts, Return(Nil()))

	g.Func().Params(Id("r").Op("*").Id(g.typeName)).Id("UnmarshalState").Params(Id("conf").Add(cty("Value"))).Id("error").Block(stmts...)

	return nil
}

func (g *Generator) writeMarshalState() error {
	stmts := []Code{
		Var().Err().Error(),
		Id("_").Op("=").Err(),
		Var().Id("state").Add(cty("Value")),
	}
	assigns, err := assignToCty(Id("r"), Id("state"), nil, g.typesStruct, 1)
	if err != nil {
		return errors.WithStack(err)
	}
	stmts = append(stmts, assigns...)
	stmts = append(stmts, Return(Id("state"), Nil()))

	g.Func().Params(Id("r").Op("*").Id(g.typeName)).Id("MarshalState").Params().Params(cty("Value"), Error()).Block(stmts...)

	return nil
}
