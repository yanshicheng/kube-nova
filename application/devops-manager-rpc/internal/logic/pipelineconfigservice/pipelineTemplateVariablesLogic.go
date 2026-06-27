package pipelineconfigservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type PipelineTemplateVariablesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewPipelineTemplateVariablesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PipelineTemplateVariablesLogic {
	return &PipelineTemplateVariablesLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *PipelineTemplateVariablesLogic) PipelineTemplateVariables(in *pb.GetByIdReq) (*pb.PipelineTemplateVariablesResp, error) {
	data, err := l.svcCtx.PipelineTemplateModel.FindOne(l.ctx, in.Id)
	if err != nil {
		l.Errorf("流水线模板查询变量失败: %v", err)
		return nil, err
	}
	if err := ensureTemplateReadAccess(l.ctx, l.svcCtx, data, in.CurrentUserId, in.CurrentRoles); err != nil {
		l.Errorf("流水线模板查询变量失败: %v", err)
		return nil, err
	}
	items, err := pipelineVariables(l.ctx, l.svcCtx, data)
	if err != nil {
		l.Errorf("流水线模板查询变量失败: %v", err)
		return nil, err
	}

	return &pb.PipelineTemplateVariablesResp{Data: items, Total: uint64(len(items))}, nil
}
