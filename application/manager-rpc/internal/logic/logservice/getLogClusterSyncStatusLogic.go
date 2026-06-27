package logservicelogic

import (
	"context"
	"errors"
	"strings"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetLogClusterSyncStatusLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetLogClusterSyncStatusLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetLogClusterSyncStatusLogic {
	return &GetLogClusterSyncStatusLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetLogClusterSyncStatusLogic) GetLogClusterSyncStatus(in *pb.GetLogClusterSyncStatusReq) (*pb.GetLogClusterSyncStatusResp, error) {
	clusterUUID := strings.TrimSpace(in.ClusterUuid)
	if clusterUUID == "" {
		return nil, errorx.Msg("集群UUID不能为空")
	}

	if _, err := l.svcCtx.OnecClusterModel.FindOneByUuid(l.ctx, clusterUUID); err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return nil, errorx.Msg("指定的集群不存在")
		}
		return nil, errorx.Msg("查询集群信息失败")
	}

	resp := &pb.GetLogClusterSyncStatusResp{
		ClusterUuid:  clusterUUID,
		HealthStatus: "unknown",
	}

	rules, err := l.svcCtx.OnecLogAlertRuleModel.SearchNoPage(l.ctx, "updated_at", false, "`cluster_uuid` = ?", clusterUUID)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return resp, nil
		}
		return nil, errorx.Msg("查询日志规则同步状态失败")
	}
	if len(rules) == 0 {
		return resp, nil
	}

	resp.TotalRules = uint64(len(rules))

	var newestFailedUpdatedAt int64
	for _, rule := range rules {
		ensureLogAlertRuleDefaults(rule)
		if rule.Enabled == 1 {
			resp.EnabledRules++
		} else {
			resp.DisabledRules++
		}

		status := strings.ToLower(strings.TrimSpace(rule.LastSyncStatus))
		switch status {
		case "success":
			if rule.Enabled == 1 {
				resp.SuccessRules++
			}
		case "failed":
			if rule.Enabled == 1 {
				resp.FailedRules++
				updatedAt := rule.UpdatedAt.Unix()
				if updatedAt >= newestFailedUpdatedAt {
					newestFailedUpdatedAt = updatedAt
					resp.LastSyncError = nullStringValue(rule.LastSyncError)
				}
			}
		case "pending", "syncing", "":
			if rule.Enabled == 1 {
				resp.PendingRules++
			}
		default:
			if rule.Enabled == 1 {
				resp.PendingRules++
			}
		}

		if updatedAt := rule.UpdatedAt.Unix(); updatedAt > resp.LastSyncAt {
			resp.LastSyncAt = updatedAt
		}
	}

	switch {
	case resp.TotalRules == 0:
		resp.HealthStatus = "unknown"
	case resp.FailedRules > 0:
		resp.HealthStatus = "degraded"
	case resp.PendingRules > 0:
		resp.HealthStatus = "syncing"
	case resp.EnabledRules == 0:
		resp.HealthStatus = "unknown"
	default:
		resp.HealthStatus = "healthy"
	}

	return resp, nil
}
