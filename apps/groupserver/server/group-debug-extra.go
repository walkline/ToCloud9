package server

import (
	"context"

	"github.com/walkline/ToCloud9/gen/group/pb"
)

func (g groupDebugLoggerMiddleware) StartReadyCheck(ctx context.Context, params *pb.StartReadyCheckRequest) (*pb.StartReadyCheckResponse, error) {
	return g.realServer.StartReadyCheck(ctx, params)
}

func (g groupDebugLoggerMiddleware) SetReadyCheckMemberState(ctx context.Context, params *pb.SetReadyCheckMemberStateRequest) (*pb.SetReadyCheckMemberStateResponse, error) {
	return g.realServer.SetReadyCheckMemberState(ctx, params)
}

func (g groupDebugLoggerMiddleware) FinishReadyCheck(ctx context.Context, params *pb.FinishReadyCheckRequest) (*pb.FinishReadyCheckResponse, error) {
	return g.realServer.FinishReadyCheck(ctx, params)
}

func (g groupDebugLoggerMiddleware) ChangeMemberSubGroup(ctx context.Context, params *pb.ChangeMemberSubGroupRequest) (*pb.ChangeMemberSubGroupResponse, error) {
	return g.realServer.ChangeMemberSubGroup(ctx, params)
}

func (g groupDebugLoggerMiddleware) SetMemberFlags(ctx context.Context, params *pb.SetMemberFlagsRequest) (*pb.SetMemberFlagsResponse, error) {
	return g.realServer.SetMemberFlags(ctx, params)
}

func (g groupDebugLoggerMiddleware) RegisterAcceptedLfgGroup(ctx context.Context, params *pb.RegisterAcceptedLfgGroupRequest) (*pb.RegisterAcceptedLfgGroupResponse, error) {
	return g.realServer.RegisterAcceptedLfgGroup(ctx, params)
}

func (g groupDebugLoggerMiddleware) RegisterMaterializedLfgGroup(ctx context.Context, params *pb.RegisterMaterializedLfgGroupRequest) (*pb.RegisterMaterializedLfgGroupResponse, error) {
	return g.realServer.RegisterMaterializedLfgGroup(ctx, params)
}

func (g groupDebugLoggerMiddleware) UpdateMemberState(ctx context.Context, params *pb.UpdateMemberStateRequest) (*pb.UpdateMemberStateResponse, error) {
	return g.realServer.UpdateMemberState(ctx, params)
}

func (g groupDebugLoggerMiddleware) BulkUpdateMemberStates(ctx context.Context, params *pb.BulkUpdateMemberStatesRequest) (*pb.BulkUpdateMemberStatesResponse, error) {
	return g.realServer.BulkUpdateMemberStates(ctx, params)
}

func (g groupDebugLoggerMiddleware) ResetInstance(ctx context.Context, params *pb.ResetInstanceRequest) (*pb.ResetInstanceResponse, error) {
	return g.realServer.ResetInstance(ctx, params)
}

func (g groupDebugLoggerMiddleware) SetInstanceBindExtension(ctx context.Context, params *pb.SetInstanceBindExtensionRequest) (*pb.SetInstanceBindExtensionResponse, error) {
	return g.realServer.SetInstanceBindExtension(ctx, params)
}
