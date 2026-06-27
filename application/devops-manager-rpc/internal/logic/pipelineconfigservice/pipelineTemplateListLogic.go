package pipelineconfigservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type PipelineTemplateListLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewPipelineTemplateListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PipelineTemplateListLogic {
	return &PipelineTemplateListLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *PipelineTemplateListLogic) PipelineTemplateList(in *pb.ListPipelineTemplateReq) (*pb.ListPipelineTemplateResp, error) {
	projectIDs, restrict, err := userProjectIDs(l.ctx, l.svcCtx, in.CurrentUserId, in.CurrentRoles)
	if err != nil {
		l.Errorf("流水线模板查询列表失败: %v", err)
		return nil, err
	}
	engineType := in.EngineType
	if engineType == "" {
		engineType = jenkinsEngineType
	}
	data, total, err := l.svcCtx.PipelineTemplateModel.List(l.ctx, model.DevopsPipelineTemplateListFilter{
		Page:               in.Page,
		PageSize:           in.PageSize,
		Name:               in.Name,
		Code:               in.Code,
		Scope:              in.Scope,
		ProjectID:          in.ProjectId,
		ProjectIDs:         projectIDs,
		RestrictProjectIDs: restrict,
		EngineType:         engineType,
		Status:             in.Status,
	})
	if err != nil {
		l.Errorf("流水线模板查询列表失败: %v", err)
		return nil, err
	}
	items := make([]*pb.DevopsPipelineTemplate, 0, len(data))
	for _, item := range data {
		items = append(items, pipelineTemplateToPb(item))
	}

	return &pb.ListPipelineTemplateResp{Data: items, Total: total}, nil
}
