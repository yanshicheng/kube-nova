package executionservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/devops-pipeline-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/devops-pipeline-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-pipeline-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type PipelineListLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewPipelineListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PipelineListLogic {
	return &PipelineListLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *PipelineListLogic) PipelineList(in *pb.ListPipelineReq) (*pb.ListPipelineResp, error) {
	projectIDs, restricted, err := accessibleProjectIDs(l.ctx, l.svcCtx, in.CurrentUserId, in.CurrentRoles)
	if err != nil {
		l.Errorf("流水线查询列表失败: %v", err)
		return nil, err
	}
	items, total, err := l.svcCtx.PipelineModel.List(l.ctx, model.DevopsPipelineListFilter{
		ProjectID:     in.ProjectId,
		ProjectIDs:    projectIDs,
		Restricted:    restricted,
		SystemID:      in.SystemId,
		EnvironmentID: in.EnvironmentId,
		Name:          in.Name,
		Code:          in.Code,
		EngineType:    in.EngineType,
		SyncStatus:    in.SyncStatus,
		LastRunStatus: in.LastRunStatus,
		TriggerMode:   in.TriggerMode,
		Status:        in.Status,
		Page:          in.Page,
		PageSize:      in.PageSize,
	})
	if err != nil {
		l.Errorf("流水线查询列表失败: %v", err)
		return nil, err
	}
	pipelineIDs := make([]string, 0, len(items))
	for _, item := range items {
		if item != nil && !item.ID.IsZero() {
			pipelineIDs = append(pipelineIDs, item.ID.Hex())
		}
	}
	latestRuns, err := l.svcCtx.PipelineRunModel.LatestByPipelines(l.ctx, pipelineIDs)
	if err != nil {
		l.Errorf("查询流水线最近运行记录失败: %v", err)
		return nil, err
	}
	data := make([]*pb.DevopsPipeline, 0, len(items))
	for _, item := range items {
		out := pipelineToPb(item)
		if latest := latestRuns[item.ID.Hex()]; latest != nil {
			fillPipelineLastRun(out, latest)
		}
		data = append(data, out)
	}

	return &pb.ListPipelineResp{Data: data, Total: total}, nil
}
