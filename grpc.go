package sdk // import "github.com/hashicorp/terraform-plugin-sdk"

import (
	"context"

	plugin "github.com/hashicorp/go-plugin"
	"github.com/pkg/errors"
	"google.golang.org/grpc"

	pb "github.com/hashicorp/terraform-plugin-sdk/tfplugin5"
)

const (
	DefaultProtocolVersion = 5
)

// Handshake is the HandshakeConfig used to configure clients and servers.
var Handshake = plugin.HandshakeConfig{
	// The ProtocolVersion is the version that must match between TF core
	// and TF plugins. This should be bumped whenever a change happens in
	// one or the other that makes it so that they can't safely communicate.
	// This could be adding a new interface value, it could be how
	// helper/schema computes diffs, etc.
	ProtocolVersion: DefaultProtocolVersion,

	// The magic cookie values should NEVER be changed.
	MagicCookieKey:   "TF_PLUGIN_MAGIC_COOKIE",
	MagicCookieValue: "d602bf8f470bc67ca7faa0386276bbdd4330efaf76d1a219cb4d6991ca9872b2",
}

func grpcServerFactory(opts []grpc.ServerOption) *grpc.Server {
	allOpts := append(opts, grpc.UnaryInterceptor(LoggingServerInterceptor()))
	return grpc.NewServer(allOpts...)
}

func ServeProvider(p Provider) error {
	plugin.Serve(&plugin.ServeConfig{
		HandshakeConfig: Handshake,
		GRPCServer:      grpcServerFactory,
		VersionedPlugins: map[int]plugin.PluginSet{
			5: map[string]plugin.Plugin{
				"provider": &grpcPlugin{
					providerServer: &GRPCProviderServer{
						Server: Server{
							Provider: p,
						},
					},
				},
			},
		},
	})

	return nil
}

type grpcPlugin struct {
	plugin.Plugin
	providerServer *GRPCProviderServer
}

func (p *grpcPlugin) GRPCServer(broker *plugin.GRPCBroker, s *grpc.Server) error {
	pb.RegisterProviderServer(s, p.providerServer)
	return nil
}

func (p *grpcPlugin) GRPCClient(ctx context.Context, broker *plugin.GRPCBroker, c *grpc.ClientConn) (interface{}, error) {
	panic("not implemented")
}

type GRPCProviderServer struct {
	Server Server
}

func (s *GRPCProviderServer) GetSchema(ctx context.Context, req *pb.GetProviderSchema_Request) (*pb.GetProviderSchema_Response, error) {
	resp, err := s.Server.GetSchema(ctx, &GetSchemaRequest{})
	if err != nil {
		return nil, errors.WithStack(err)
	}

	p, err := pbSchema(resp.Provider)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	resources, err := pbMapSchema(resp.ResourceSchemas)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	dataSources, err := pbMapSchema(resp.DataSourceSchemas)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return &pb.GetProviderSchema_Response{
		Provider:          p,
		DataSourceSchemas: dataSources,
		ResourceSchemas:   resources,
	}, nil
}

func (s *GRPCProviderServer) PrepareProviderConfig(ctx context.Context, req *pb.PrepareProviderConfig_Request) (*pb.PrepareProviderConfig_Response, error) {
	resp, err := s.Server.PrepareProviderConfig(ctx, &PrepareProviderConfigRequest{
		Config: req.Config.Msgpack,
	})
	if err != nil {
		return nil, errors.WithStack(err)
	}
	pbDiags, err := pbDiagnostics(resp.Diagnostics)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &pb.PrepareProviderConfig_Response{
		PreparedConfig: pbDynamicValue(resp.PreparedConfig),
		Diagnostics:    pbDiags,
	}, nil
}

func (s *GRPCProviderServer) ValidateResourceTypeConfig(ctx context.Context, req *pb.ValidateResourceTypeConfig_Request) (*pb.ValidateResourceTypeConfig_Response, error) {
	resp, err := s.Server.ValidateResourceTypeConfig(ctx, &ValidateResourceTypeConfigRequest{
		TypeName: req.TypeName,
		Config:   req.Config.Msgpack,
	})
	if err != nil {
		return nil, errors.WithStack(err)
	}
	pbDiags, err := pbDiagnostics(resp.Diagnostics)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &pb.ValidateResourceTypeConfig_Response{
		Diagnostics: pbDiags,
	}, nil
}

func (s *GRPCProviderServer) ValidateDataSourceConfig(ctx context.Context, req *pb.ValidateDataSourceConfig_Request) (*pb.ValidateDataSourceConfig_Response, error) {
	resp, err := s.Server.ValidateDataSourceConfig(ctx, &ValidateDataSourceConfigRequest{
		TypeName: req.TypeName,
		Config:   req.Config.Msgpack,
	})
	if err != nil {
		return nil, errors.WithStack(err)
	}
	pbDiags, err := pbDiagnostics(resp.Diagnostics)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &pb.ValidateDataSourceConfig_Response{
		Diagnostics: pbDiags,
	}, nil
}

func (s *GRPCProviderServer) UpgradeResourceState(context.Context, *pb.UpgradeResourceState_Request) (*pb.UpgradeResourceState_Response, error) {
	panic("UpgradeResourceState not implemented")
}

func (s *GRPCProviderServer) Configure(ctx context.Context, req *pb.Configure_Request) (*pb.Configure_Response, error) {
	resp, err := s.Server.Configure(ctx, &ConfigureRequest{
		Config:           req.Config.Msgpack,
		TerraformVersion: req.TerraformVersion,
	})
	if err != nil {
		return nil, errors.WithStack(err)
	}
	pbDiags, err := pbDiagnostics(resp.Diagnostics)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &pb.Configure_Response{
		Diagnostics: pbDiags,
	}, nil
}

func (s *GRPCProviderServer) ReadResource(ctx context.Context, req *pb.ReadResource_Request) (*pb.ReadResource_Response, error) {
	resp, err := s.Server.ReadResource(ctx, &ReadResourceRequest{
		TypeName:     req.TypeName,
		CurrentState: req.CurrentState.Msgpack,
	})
	if err != nil {
		return nil, errors.WithStack(err)
	}
	pbDiags, err := pbDiagnostics(resp.Diagnostics)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &pb.ReadResource_Response{
		Diagnostics: pbDiags,
		NewState:    pbDynamicValue(resp.NewState),
	}, nil
}

func (s *GRPCProviderServer) PlanResourceChange(ctx context.Context, req *pb.PlanResourceChange_Request) (*pb.PlanResourceChange_Response, error) {
	resp, err := s.Server.PlanResourceChange(ctx, &PlanResourceChangeRequest{
		TypeName:         req.TypeName,
		PriorState:       req.PriorState.Msgpack,
		Config:           req.Config.Msgpack,
		ProposedNewState: req.ProposedNewState.Msgpack,
	})
	if err != nil {
		return nil, errors.WithStack(err)
	}
	pbDiags, err := pbDiagnostics(resp.Diagnostics)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	pbRequiresReplace, err := pbAttributePaths(resp.RequiresReplace)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	// TODO: if no update method, and any changes, force new?
	return &pb.PlanResourceChange_Response{
		Diagnostics:     pbDiags,
		PlannedState:    pbDynamicValue(resp.PlannedState),
		RequiresReplace: pbRequiresReplace,
	}, nil
}

func (s *GRPCProviderServer) ApplyResourceChange(ctx context.Context, req *pb.ApplyResourceChange_Request) (*pb.ApplyResourceChange_Response, error) {
	resp, err := s.Server.ApplyResourceChange(ctx, &ApplyResourceChangeRequest{
		TypeName:     req.TypeName,
		PlannedState: req.PlannedState.Msgpack,
		PriorState:   req.PriorState.Msgpack,
	})
	if err != nil {
		return nil, errors.WithStack(err)
	}
	pbDiags, err := pbDiagnostics(resp.Diagnostics)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &pb.ApplyResourceChange_Response{
		Diagnostics: pbDiags,
		NewState:    pbDynamicValue(resp.NewState),
	}, nil
}

func (s *GRPCProviderServer) ImportResourceState(context.Context, *pb.ImportResourceState_Request) (*pb.ImportResourceState_Response, error) {
	panic("ImportResourceState not implemented")
}

func (s *GRPCProviderServer) ReadDataSource(ctx context.Context, req *pb.ReadDataSource_Request) (*pb.ReadDataSource_Response, error) {
	resp, err := s.Server.ReadDataSource(ctx, &ReadDataSourceRequest{
		TypeName: req.TypeName,
		Config:   req.Config.Msgpack,
	})
	if err != nil {
		return nil, errors.WithStack(err)
	}
	pbDiags, err := pbDiagnostics(resp.Diagnostics)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &pb.ReadDataSource_Response{
		Diagnostics: pbDiags,
		State:       pbDynamicValue(resp.State),
	}, nil
}

func (s *GRPCProviderServer) Stop(ctx context.Context, req *pb.Stop_Request) (*pb.Stop_Response, error) {
	err := s.Server.Stop(ctx)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &pb.Stop_Response{}, nil
}
