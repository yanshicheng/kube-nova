package executionservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/devops-pipeline-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/devops-pipeline-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-pipeline-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type PipelineGetLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

type latestPipelineRunResult struct {
	run *model.DevopsPipelineRun
	err error
}

func NewPipelineGetLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PipelineGetLogic {
	return &PipelineGetLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *PipelineGetLogic) PipelineGet(in *pb.GetPipelineReq) (*pb.GetPipelineResp, error) {
	data, err := l.svcCtx.PipelineModel.FindOne(l.ctx, in.Id)
	if err != nil {
		l.Errorf("流水线查询详情失败: %v", err)
		return nil, err
	}
	accessCh := make(chan error, 1)
	latestCh := make(chan latestPipelineRunResult, 1)
	go func() {
		accessCh <- ensurePipelineAccess(l.ctx, l.svcCtx, data, in.CurrentUserId, in.CurrentRoles)
	}()
	go func() {
		latest, err := l.svcCtx.PipelineRunModel.FindLatestByPipeline(l.ctx, data.ID.Hex())
		latestCh <- latestPipelineRunResult{run: latest, err: err}
	}()
	if err := <-accessCh; err != nil {
		l.Errorf("流水线查询详情失败: %v", err)
		return nil, err
	}

	out := pipelineToPb(data)
	if latest := <-latestCh; latest.err == nil {
		fillPipelineLastRun(out, latest.run)
	}
	return &pb.GetPipelineResp{Data: out}, nil
}
