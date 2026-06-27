package pipelineconfigservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type PipelineEnvironmentListLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewPipelineEnvironmentListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PipelineEnvironmentListLogic {
	return &PipelineEnvironmentListLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *PipelineEnvironmentListLogic) PipelineEnvironmentList(in *pb.ListPipelineEnvironmentReq) (*pb.ListPipelineEnvironmentResp, error) {
	items, total, err := l.svcCtx.PipelineEnvModel.List(l.ctx, model.DevopsPipelineEnvironmentListFilter{
		Name:     in.Name,
		Code:     in.Code,
		Status:   in.Status,
		Page:     in.Page,
		PageSize: in.PageSize,
	})
	if err != nil {
		l.Errorf("流水线环境查询列表失败: %v", err)
		return nil, err
	}
	data := make([]*pb.DevopsPipelineEnvironment, 0, len(items))
	for _, item := range items {
		data = append(data, pipelineEnvironmentToPb(item))
	}

	return &pb.ListPipelineEnvironmentResp{Data: data, Total: total}, nil
}
