package sdk

import (
	"context"

	"github.com/pkg/errors"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/msgpack"
)

type Server struct {
	Provider Provider
}

type GetSchemaRequest struct {
}

type GetSchemaResponse struct {
	Provider          Schema
	DataSourceSchemas map[string]Schema
	ResourceSchemas   map[string]Schema
}

func (s *Server) GetSchema(ctx context.Context, req *GetSchemaRequest) (*GetSchemaResponse, error) {
	return &GetSchemaResponse{
		Provider:          s.Provider.Schema(),
		DataSourceSchemas: s.Provider.DataSourceSchemas(),
		ResourceSchemas:   s.Provider.ResourceSchemas(),
	}, nil
}

type PrepareProviderConfigRequest struct {
	Config []byte
}

type PrepareProviderConfigResponse struct {
	PreparedConfig []byte
	Diagnostics    Diagnostics
}

func (s *Server) PrepareProviderConfig(ctx context.Context, req *PrepareProviderConfigRequest) (*PrepareProviderConfigResponse, error) {
	blockType := blockType(s.Provider)
	config, err := msgpack.Unmarshal(req.Config, blockType)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	err = unmarshalState(s.Provider, config)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	var diags Diagnostics
	if v, ok := s.Provider.(Validator); ok {
		err := v.Validate()
		diags, err = errorOrDiagnostics(err)
		if err != nil {
			return nil, errors.WithStack(err)
		}
		if diags.IsError() {
			return &PrepareProviderConfigResponse{
				Diagnostics: diags,
			}, nil
		}
	}

	state, err := s.Provider.MarshalState()
	if err != nil {
		return nil, errors.WithStack(err)
	}

	data, err := msgpack.Marshal(state, blockType)
	if err != nil {
		return nil, errors.Wrap(err, "unable to marshal state for provider block")
	}

	return &PrepareProviderConfigResponse{
		PreparedConfig: data,
		Diagnostics:    diags,
	}, nil
}

type ValidateResourceTypeConfigRequest struct {
	TypeName string
	Config   []byte
}

type ValidateResourceTypeConfigResponse struct {
	Diagnostics Diagnostics
}

func (s *Server) ValidateResourceTypeConfig(ctx context.Context, req *ValidateResourceTypeConfigRequest) (*ValidateResourceTypeConfigResponse, error) {
	r := s.Provider.ResourceFactory(req.TypeName)
	blockType := blockType(r)
	config, err := msgpack.Unmarshal(req.Config, blockType)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	err = unmarshalState(r, config)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	var diags Diagnostics
	if v, ok := r.(Validator); ok {
		err := v.Validate()
		diags, err = errorOrDiagnostics(err)
		if err != nil {
			return nil, errors.WithStack(err)
		}
	}

	return &ValidateResourceTypeConfigResponse{
		Diagnostics: diags,
	}, nil
}

type ValidateDataSourceConfigRequest struct {
	TypeName string
	Config   []byte
}

type ValidateDataSourceConfigResponse struct {
	Diagnostics Diagnostics
}

func (s *Server) ValidateDataSourceConfig(ctx context.Context, req *ValidateDataSourceConfigRequest) (*ValidateDataSourceConfigResponse, error) {
	ds := s.Provider.DataSourceFactory(req.TypeName)
	blockType := blockType(ds)
	config, err := msgpack.Unmarshal(req.Config, blockType)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	err = unmarshalState(ds, config)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	var diags Diagnostics
	if v, ok := ds.(Validator); ok {
		err := v.Validate()
		diags, err = errorOrDiagnostics(err)
		if err != nil {
			return nil, errors.WithStack(err)
		}
	}

	return &ValidateDataSourceConfigResponse{
		Diagnostics: diags,
	}, nil
}

type UpgradeResourceStateRequest struct {
}

type UpgradeResourceStateResponse struct {
}

func (s *Server) UpgradeResourceState(context.Context, *UpgradeResourceStateRequest) (*UpgradeResourceStateResponse, error) {
	panic("UpgradeResourceState not implemented")
}

type ConfigureRequest struct {
	Config           []byte
	TerraformVersion string
}

type ConfigureResponse struct {
	Diagnostics Diagnostics
}

func (s *Server) Configure(ctx context.Context, req *ConfigureRequest) (*ConfigureResponse, error) {
	blockType := blockType(s.Provider)
	config, err := msgpack.Unmarshal(req.Config, blockType)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	err = unmarshalState(s.Provider, config)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	err = s.Provider.Configure(ctx, req.TerraformVersion)
	diags, err := errorOrDiagnostics(err)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return &ConfigureResponse{
		Diagnostics: diags,
	}, nil
}

type ReadResourceRequest struct {
	TypeName     string
	CurrentState []byte
}

type ReadResourceResponse struct {
	Diagnostics Diagnostics
	NewState    []byte
}

func (s *Server) ReadResource(ctx context.Context, req *ReadResourceRequest) (*ReadResourceResponse, error) {
	r := s.Provider.ResourceFactory(req.TypeName)
	blockType := blockType(r)
	current, err := msgpack.Unmarshal(req.CurrentState, blockType)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	err = unmarshalState(r, current)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	err = r.Read(ctx)
	if _, ok := err.(*doesNotExistError); ok {
		// resource does not exist, return empty state
		state := cty.NullVal(blockType)
		data, err := msgpack.Marshal(state, blockType)
		if err != nil {
			return nil, errors.Wrapf(err, "unable to marshal state for resource: %s", req.TypeName)
		}

		return &ReadResourceResponse{
			NewState: data,
		}, nil
	}
	diags, err := errorOrDiagnostics(err)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	if diags.IsError() {
		return &ReadResourceResponse{
			Diagnostics: diags,
		}, nil
	}

	state, err := r.MarshalState()
	if err != nil {
		return nil, errors.WithStack(err)
	}

	data, err := msgpack.Marshal(state, blockType)
	if err != nil {
		return nil, errors.Wrapf(err, "unable to marshal state for resource: %s", req.TypeName)
	}

	return &ReadResourceResponse{
		Diagnostics: diags,
		NewState:    data,
	}, nil
}

type change struct {
	Path      cty.Path
	Attribute Attribute
	From      cty.Value
	To        cty.Value
}

// TODO: replace this wity cty's diff stuff?
func changes(r Resource, from, to cty.Value) ([]change, error) {
	schemaBlock := r.Schema().Block

	var changes []change

	// Walk to to see if anything has changed and
	// would require replacement (force new)
	// TODO: swap this once cty implements NewDiff...
	err := cty.Walk(to, func(path cty.Path, toVal cty.Value) (bool, error) {
		if len(path) == 0 {
			// skip root
			return true, nil
		}

		schemaAtt, err := schemaBlock.ApplyPath(path)
		if err != nil {
			return false, errors.Wrapf(err, "unable to apply path to schema block for path: %#v", path)
		}
		if schemaAtt == nil {
			return false, errors.Errorf("path not found in schema: %v", path)
		}

		fromVal, err := path.Apply(from)
		if err != nil {
			return false, errors.Wrap(err, "unable to apply path to prior state")
		}

		equal := fromVal.Equals(toVal)
		if equal.IsKnown() && equal.True() {
			return false, nil
		}

		changes = append(changes, change{
			Path:      path,
			Attribute: *schemaAtt,
			From:      fromVal,
			To:        toVal,
		})
		return false, nil
	})
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return changes, nil
}

type PlanResourceChangeRequest struct {
	TypeName         string
	Config           []byte
	PriorState       []byte
	ProposedNewState []byte
}

type PlanResourceChangeResponse struct {
	Diagnostics     Diagnostics
	RequiresReplace []cty.Path
	PlannedState    []byte
}

func (s *Server) PlanResourceChange(ctx context.Context, req *PlanResourceChangeRequest) (*PlanResourceChangeResponse, error) {
	r := s.Provider.ResourceFactory(req.TypeName)
	blockType := blockType(r)
	prior, err := msgpack.Unmarshal(req.PriorState, blockType)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	config, err := msgpack.Unmarshal(req.Config, blockType)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	proposed, err := msgpack.Unmarshal(req.ProposedNewState, blockType)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	if proposed.IsNull() {
		// short circuit, this is a destroy
		return &PlanResourceChangeResponse{
			PlannedState: req.ProposedNewState,
		}, nil
	}
	err = unmarshalState(r, proposed)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	//TODO: validation?

	planned, err := r.MarshalState()
	if err != nil {
		return nil, errors.WithStack(err)
	}

	schemaBlock := r.Schema().Block
	planned, err = cty.Transform(planned, func(path cty.Path, v cty.Value) (cty.Value, error) {
		if len(path) == 0 {
			// skip root
			return v, nil
		}

		schemaAtt, err := schemaBlock.ApplyPath(path)
		if err != nil {
			return cty.NilVal, errors.Wrapf(err, "unable to apply path to schema block for path: %#v", path)
		}
		if schemaAtt == nil {
			return cty.NilVal, errors.Errorf("path not found in schema: %v", path)
		}

		// short circuit if only computed:
		if schemaAtt.Computed && !schemaAtt.IsArgument() {
			return cty.UnknownVal(v.Type()), nil
		}

		// TODO: is this necessary? I think they aren't propagate
		// via PopulateConfig/SaveState
		// mark all unknown proposed values as unknown in planned
		proposedVal, err := path.Apply(proposed)
		if err != nil {
			return cty.NilVal, errors.Wrap(err, "unable to apply path to proposed state")
		}

		if !proposedVal.IsKnown() {
			return cty.UnknownVal(v.Type()), nil
		}

		configVal, err := path.Apply(config)
		if err != nil {
			return cty.NilVal, errors.Wrap(err, "unable to apply path to config state")
		}

		if schemaAtt.Computed && configVal.IsNull() {
			// this is an argument since it passed the earlier short circuit
			return cty.UnknownVal(v.Type()), nil
		}

		return v, nil
	})
	if err != nil {
		return nil, errors.WithStack(err)
	}

	needsApply := false
	if prior.IsNull() {
		needsApply = true
	} else {
		potentialChanges, err := changes(r, prior, planned)
		if err != nil {
			return nil, errors.WithStack(err)
		}
		for _, c := range potentialChanges {
			if !c.Attribute.IsArgument() {
				//only check user supplied values
				continue
			}

			needsApply = true
			break
		}
	}

	if !needsApply {
		return &PlanResourceChangeResponse{
			PlannedState: req.PriorState,
		}, nil
	}

	data, err := msgpack.Marshal(planned, blockType)
	if err != nil {
		return nil, errors.Wrapf(err, "unable to marshal state for resource: %s", req.TypeName)
	}

	var requiresReplace []cty.Path
	if !prior.IsNull() {
		potentialChanges, err := changes(r, prior, planned)
		if err != nil {
			return nil, errors.WithStack(err)
		}

		_, isUpdater := r.(Updater)

		for _, c := range potentialChanges {
			if !c.Attribute.IsArgument() {
				//only check user supplied values
				continue
			}

			if !isUpdater {
				requiresReplace = append(requiresReplace, c.Path)
				continue
			}

			if c.Attribute.ForceNew {
				requiresReplace = append(requiresReplace, c.Path)
				continue
			}
		}
	}

	// TODO: if no update method, and any changes, force new?
	return &PlanResourceChangeResponse{
		PlannedState:    data,
		RequiresReplace: requiresReplace,
	}, nil
}

type ApplyResourceChangeRequest struct {
	TypeName     string
	PlannedState []byte
	PriorState   []byte
}

type ApplyResourceChangeResponse struct {
	Diagnostics Diagnostics
	NewState    []byte
}

func (s *Server) ApplyResourceChange(ctx context.Context, req *ApplyResourceChangeRequest) (*ApplyResourceChangeResponse, error) {
	r := s.Provider.ResourceFactory(req.TypeName)
	blockType := blockType(r)
	planned, err := msgpack.Unmarshal(req.PlannedState, blockType)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	prior, err := msgpack.Unmarshal(req.PriorState, blockType)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	if planned.IsNull() {
		// this is a delete, so can skip validation, and need to apply prior state
		err = unmarshalState(r, prior)
		if err != nil {
			return nil, errors.WithStack(err)
		}

		err = r.Delete(ctx)
		diags, err := errorOrDiagnostics(err)
		if err != nil {
			return nil, errors.WithStack(err)
		}
		if diags.IsError() {
			return &ApplyResourceChangeResponse{
				Diagnostics: diags,
			}, nil
		}

		return &ApplyResourceChangeResponse{
			Diagnostics: diags,
			NewState:    req.PlannedState,
		}, nil
	}

	err = unmarshalState(r, planned)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	var diags Diagnostics
	// re-validate again now that we have more info, only for create/update
	if v, ok := r.(Validator); ok && !planned.IsNull() {
		err := v.Validate()
		diags, err = errorOrDiagnostics(err)
		if err != nil {
			return nil, errors.WithStack(err)
		}
		if diags.IsError() {
			return &ApplyResourceChangeResponse{
				Diagnostics: diags,
			}, nil
		}
	}

	// if planned.IsWhollyKnown() && !planned.IsNull() {
	// 	return &pb.ApplyResourceChange_Response{
	// 		NewState:    req.PlannedState,
	// 		Diagnostics: pbDiags,
	// 	}, nil
	// }

	err = nil
	switch {
	case prior.IsNull():
		err = r.Create(ctx)
	case planned.IsNull():
		// should not get here
		panic("unexpected null planned state")
	default:
		updater, ok := r.(Updater)
		if !ok {
			return nil, errors.Errorf("attempting to update something with no Update implementation")
		}
		err = updater.Update(ctx)
	}
	appendDiags, err := errorOrDiagnostics(err)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	diags = append(diags, appendDiags...)
	if diags.IsError() {
		return &ApplyResourceChangeResponse{
			Diagnostics: diags,
		}, nil
	}

	state, err := r.MarshalState()
	if err != nil {
		return nil, errors.WithStack(err)
	}

	data, err := msgpack.Marshal(state, blockType)
	if err != nil {
		return nil, errors.Wrapf(err, "unable to marshal state for resource: %s", req.TypeName)
	}

	return &ApplyResourceChangeResponse{
		Diagnostics: diags,
		NewState:    data,
	}, nil
}

type ImportResourceStateRequest struct {
}

type ImportResourceStateResponse struct {
}

func (s *Server) ImportResourceState(context.Context, *ImportResourceStateRequest) (*ImportResourceStateResponse, error) {
	panic("ImportResourceState not implemented")
}

type ReadDataSourceRequest struct {
	TypeName string
	Config   []byte
}

type ReadDataSourceResponse struct {
	Diagnostics Diagnostics
	State       []byte
}

func (s *Server) ReadDataSource(ctx context.Context, req *ReadDataSourceRequest) (*ReadDataSourceResponse, error) {
	ds := s.Provider.DataSourceFactory(req.TypeName)
	blockType := blockType(ds)

	config, err := msgpack.Unmarshal(req.Config, blockType)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	err = unmarshalState(ds, config)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	err = ds.Read(ctx)
	diags, err := errorOrDiagnostics(err)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	if diags.IsError() {
		return &ReadDataSourceResponse{
			Diagnostics: diags,
		}, nil
	}

	state, err := ds.MarshalState()
	if err != nil {
		return nil, errors.WithStack(err)
	}

	data, err := msgpack.Marshal(state, blockType)
	if err != nil {
		return nil, errors.Wrapf(err, "unable to marshal state for data source: %s", req.TypeName)
	}

	return &ReadDataSourceResponse{
		Diagnostics: diags,
		State:       data,
	}, nil
}

func (s *Server) Stop(ctx context.Context) error {
	return s.Provider.Stop(ctx)
}
