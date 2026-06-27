package pipelineconfigservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type DeleteStepChannelParamMappingLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewDeleteStepChannelParamMappingLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteStepChannelParamMappingLogic {
	return &DeleteStepChannelParamMappingLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *DeleteStepChannelParamMappingLogic) DeleteStepChannelParamMapping(in *pb.DeleteByIdReq) (*pb.EmptyResp, error) {
	if err := l.svcCtx.StepChannelParamModel.DeleteSoft(l.ctx, in.Id, in.UpdatedBy); err != nil {
		l.Errorf("删除步骤渠道参数映射失败: %v", err)
		return nil, err
	}
	if err := refreshStepChannelLookup(l.ctx, l.svcCtx); err != nil {
		l.Errorf("刷新步骤渠道参数映射缓存失败: %v", err)
		return nil, err
	}
	return &pb.EmptyResp{}, nil
}
