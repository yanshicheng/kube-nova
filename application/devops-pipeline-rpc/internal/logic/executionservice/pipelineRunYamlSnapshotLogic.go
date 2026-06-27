package executionservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/devops-pipeline-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-pipeline-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type PipelineRunYamlSnapshotLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewPipelineRunYamlSnapshotLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PipelineRunYamlSnapshotLogic {
	return &PipelineRunYamlSnapshotLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *PipelineRunYamlSnapshotLogic) PipelineRunYamlSnapshot(in *pb.PipelineRunYamlSnapshotReq) (*pb.PipelineRunYamlSnapshotResp, error) {
	run, err := l.svcCtx.PipelineRunModel.FindOne(l.ctx, in.RunId)
	if err != nil {
		l.Errorf("查询流水线 YAML 快照失败: %v", err)
		return nil, err
	}
	if err := ensureProjectAccess(l.ctx, l.svcCtx, run.ProjectID, in.CurrentUserId, in.CurrentRoles); err != nil {
		l.Errorf("查询流水线 YAML 快照失败: %v", err)
		return nil, err
	}

	return &pb.PipelineRunYamlSnapshotResp{
		RunId:                         run.ID.Hex(),
		PipelineId:                    run.PipelineID,
		EngineType:                    run.EngineType,
		TektonNamespace:               run.TektonNamespace,
		TektonPipelineName:            run.TektonPipelineName,
		TektonPipelineRunName:         run.TektonPipelineRunName,
		TektonPipelineYamlSnapshot:    run.TektonPipelineYamlSnapshot,
		TektonPipelineRunYamlSnapshot: run.TektonPipelineRunYamlSnapshot,
		PipelineSnapshot:              run.PipelineSnapshot,
	}, nil
}
