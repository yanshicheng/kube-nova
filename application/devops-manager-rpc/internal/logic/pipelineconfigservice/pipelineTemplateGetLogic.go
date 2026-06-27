package pipelineconfigservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type PipelineTemplateGetLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewPipelineTemplateGetLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PipelineTemplateGetLogic {
	return &PipelineTemplateGetLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *PipelineTemplateGetLogic) PipelineTemplateGet(in *pb.GetByIdReq) (*pb.GetPipelineTemplateResp, error) {
	data, err := l.svcCtx.PipelineTemplateModel.FindOne(l.ctx, in.Id)
	if err != nil {
		l.Errorf("流水线模板查询详情失败: %v", err)
		return nil, err
	}
	if err := ensureTemplateReadAccess(l.ctx, l.svcCtx, data, in.CurrentUserId, in.CurrentRoles); err != nil {
		l.Errorf("流水线模板查询详情失败: %v", err)
		return nil, err
	}

	return &pb.GetPipelineTemplateResp{Data: pipelineTemplateToPb(data)}, nil
}
