// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package pipeline

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/devops-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/devops-pipeline-rpc/client/executionservice"

	"github.com/zeromicro/go-zero/core/logx"
)

type DevopsPipelineRunYamlSnapshotLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 查询流水线 YAML 快照
func NewDevopsPipelineRunYamlSnapshotLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DevopsPipelineRunYamlSnapshotLogic {
	return &DevopsPipelineRunYamlSnapshotLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DevopsPipelineRunYamlSnapshotLogic) DevopsPipelineRunYamlSnapshot(req *types.DevopsPipelineRunYamlSnapshotRequest) (resp *types.DevopsPipelineRunYamlSnapshotResponse, err error) {
	result, err := l.svcCtx.PipelineExecRpc.PipelineRunYamlSnapshot(l.ctx, &executionservice.PipelineRunYamlSnapshotReq{
		RunId:         req.RunId,
		CurrentUserId: currentUserID(l.ctx),
		CurrentRoles:  currentRoles(l.ctx),
	})
	if err != nil {
		return nil, err
	}

	return &types.DevopsPipelineRunYamlSnapshotResponse{
		RunId:                         result.RunId,
		PipelineId:                    result.PipelineId,
		EngineType:                    result.EngineType,
		TektonNamespace:               result.TektonNamespace,
		TektonPipelineName:            result.TektonPipelineName,
		TektonPipelineRunName:         result.TektonPipelineRunName,
		TektonPipelineYamlSnapshot:    result.TektonPipelineYamlSnapshot,
		TektonPipelineRunYamlSnapshot: result.TektonPipelineRunYamlSnapshot,
		PipelineSnapshot:              result.PipelineSnapshot,
	}, nil
}
