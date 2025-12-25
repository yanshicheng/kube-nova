package alert

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/manager-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetAlertInstanceLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 根据ID获取告警实例详细信息
func NewGetAlertInstanceLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetAlertInstanceLogic {
	return &GetAlertInstanceLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetAlertInstanceLogic) GetAlertInstance(req *types.DefaultIdRequest) (resp *types.AlertInstance, err error) {
	// 调用RPC服务获取告警实例详情
	result, err := l.svcCtx.ManagerRpc.AlertInstancesByIdGet(l.ctx, &pb.GetAlertInstancesByIdReq{
		Id: req.Id,
	})

	if err != nil {
		l.Errorf("获取告警实例详情失败: %v", err)
		return nil, fmt.Errorf("获取告警实例详情失败: %v", err)
	}

	// 转换为 API 类型
	return &types.AlertInstance{
		Id:                result.Data.Id,
		Instance:          result.Data.Instance,
		Fingerprint:       result.Data.Fingerprint,
		ClusterUuid:       result.Data.ClusterUuid,
		ClusterName:       result.Data.ClusterName,
		ProjectId:         result.Data.ProjectId,
		ProjectName:       result.Data.ProjectName,
		WorkspaceId:       result.Data.WorkspaceId,
		WorkspaceName:     result.Data.WorkspaceName,
		AlertName:         result.Data.AlertName,
		Severity:          result.Data.Severity,
		Status:            result.Data.Status,
		Labels:            result.Data.Labels,
		Annotations:       result.Data.Annotations,
		GeneratorUrl:      result.Data.GeneratorUrl,
		StartsAt:          result.Data.StartsAt,
		EndsAt:            result.Data.EndsAt,
		ResolvedAt:        result.Data.ResolvedAt,
		Duration:          result.Data.Duration,
		RepeatCount:       result.Data.RepeatCount,
		NotifiedGroups:    result.Data.NotifiedGroups,
		NotificationCount: result.Data.NotificationCount,
		LastNotifiedAt:    result.Data.LastNotifiedAt,
		CreatedBy:         result.Data.CreatedBy,
		UpdatedBy:         result.Data.UpdatedBy,
		CreatedAt:         result.Data.CreatedAt,
		UpdatedAt:         result.Data.UpdatedAt,
	}, nil
}
