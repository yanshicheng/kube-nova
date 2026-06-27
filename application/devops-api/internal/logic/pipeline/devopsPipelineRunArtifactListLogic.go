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

type DevopsPipelineRunArtifactListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 查询流水线运行产物
func NewDevopsPipelineRunArtifactListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DevopsPipelineRunArtifactListLogic {
	return &DevopsPipelineRunArtifactListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DevopsPipelineRunArtifactListLogic) DevopsPipelineRunArtifactList(req *types.ListDevopsPipelineRunArtifactRequest) (resp *types.ListDevopsPipelineRunArtifactResponse, err error) {
	result, err := l.svcCtx.PipelineExecRpc.PipelineRunArtifactList(l.ctx, &executionservice.PipelineRunArtifactListReq{
		RunId:         req.RunId,
		CurrentUserId: currentUserID(l.ctx),
		CurrentRoles:  currentRoles(l.ctx),
	})
	if err != nil {
		return nil, err
	}
	items := make([]types.DevopsPipelineRunArtifact, 0, len(result.Data))
	for _, item := range result.Data {
		if item == nil {
			continue
		}
		items = append(items, types.DevopsPipelineRunArtifact{
			Id:           item.Id,
			RunId:        item.RunId,
			StageId:      item.StageId,
			StepId:       item.StepId,
			Name:         item.Name,
			Type:         item.Type,
			Bucket:       item.Bucket,
			ObjectKey:    item.ObjectKey,
			Size:         item.Size,
			ContentType:  item.ContentType,
			Status:       item.Status,
			ErrorMessage: item.ErrorMessage,
			Url:          item.Url,
			CreatedAt:    item.CreatedAt,
		})
	}

	return &types.ListDevopsPipelineRunArtifactResponse{Items: items}, nil
}
