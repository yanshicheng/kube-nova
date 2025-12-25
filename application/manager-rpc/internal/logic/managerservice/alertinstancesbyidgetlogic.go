package managerservicelogic

import (
	"context"
	"database/sql"
	"errors"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type AlertInstancesByIdGetLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAlertInstancesByIdGetLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AlertInstancesByIdGetLogic {
	return &AlertInstancesByIdGetLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *AlertInstancesByIdGetLogic) AlertInstancesByIdGet(in *pb.GetAlertInstancesByIdReq) (*pb.GetAlertInstancesByIdResp, error) {
	// 查询告警实例
	alertInstance, err := l.svcCtx.AlertInstancesModel.FindOne(l.ctx, in.Id)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return nil, errorx.Msg("告警实例不存在")
		}
		return nil, errorx.Msg("查询告警实例失败")
	}

	// 转换为 pb 格式
	return &pb.GetAlertInstancesByIdResp{
		Data: &pb.AlertInstances{
			Id:                uint64(alertInstance.Id),
			Instance:          alertInstance.Instance,
			Fingerprint:       alertInstance.Fingerprint,
			ClusterUuid:       alertInstance.ClusterUuid,
			ClusterName:       alertInstance.ClusterName,
			ProjectId:         alertInstance.ProjectId,
			ProjectName:       alertInstance.ProjectName,
			WorkspaceId:       alertInstance.WorkspaceId,
			WorkspaceName:     alertInstance.WorkspaceName,
			AlertName:         alertInstance.AlertName,
			Severity:          alertInstance.Severity,
			Status:            alertInstance.Status,
			Labels:            alertInstance.Labels,
			Annotations:       alertInstance.Annotations,
			GeneratorUrl:      alertInstance.GeneratorUrl,
			StartsAt:          alertInstance.StartsAt.Unix(),
			EndsAt:            getTimeValue(alertInstance.EndsAt),
			ResolvedAt:        getTimeValue(alertInstance.ResolvedAt),
			Duration:          int64(alertInstance.Duration),
			RepeatCount:       int64(alertInstance.RepeatCount),
			NotifiedGroups:    alertInstance.NotifiedGroups,
			NotificationCount: int64(alertInstance.NotificationCount),
			LastNotifiedAt:    getTimeValue(alertInstance.LastNotifiedAt),
			CreatedBy:         alertInstance.CreatedBy,
			UpdatedBy:         alertInstance.UpdatedBy,
			CreatedAt:         alertInstance.CreatedAt.Unix(),
			UpdatedAt:         alertInstance.UpdatedAt.Unix(),
		},
	}, nil
}

// 辅助函数：处理 sql.NullTime
func getTimeValue(t sql.NullTime) int64 {
	if t.Valid {
		return t.Time.Unix()
	}
	return 0
}
