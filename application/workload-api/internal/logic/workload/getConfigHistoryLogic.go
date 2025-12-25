package workload

import (
	"context"
	"fmt"
	"strings"

	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"
	types2 "github.com/yanshicheng/kube-nova/common/k8smanager/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetConfigHistoryLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetConfigHistoryLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetConfigHistoryLogic {
	return &GetConfigHistoryLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetConfigHistoryLogic) GetConfigHistory(req *types.DefaultIdRequest) (resp []types.ConfigHistoryInfo, err error) {
	client, versionDetail, err := getResourceClusterClient(l.ctx, l.svcCtx, req.Id)
	if err != nil {
		l.Errorf("获取集群客户端失败: %v", err)
		return nil, fmt.Errorf("获取集群客户端失败")
	}

	resourceType := strings.ToUpper(versionDetail.ResourceType)

	switch resourceType {
	case "STATEFULSET":
		configs, err := client.StatefulSet().GetConfigHistory(versionDetail.Namespace, versionDetail.ResourceName)
		if err != nil {
			l.Errorf("获取 StatefulSet 配置历史失败: %v", err)
			return nil, fmt.Errorf("获取配置历史失败")
		}
		resp = convertToConfigHistoryInfo(configs)
	case "DAEMONSET":
		configs, err := client.DaemonSet().GetConfigHistory(versionDetail.Namespace, versionDetail.ResourceName)
		if err != nil {
			l.Errorf("获取 DaemonSet 配置历史失败: %v", err)
			return nil, fmt.Errorf("获取配置历史失败")
		}
		resp = convertToConfigHistoryInfo(configs)
	default:
		return nil, fmt.Errorf("资源类型 %s 不支持查询配置历史", resourceType)
	}

	return resp, nil
}

func convertToConfigHistoryInfo(configs []types2.ConfigHistoryInfo) []types.ConfigHistoryInfo {
	result := make([]types.ConfigHistoryInfo, 0, len(configs))
	for _, cfg := range configs {
		result = append(result, types.ConfigHistoryInfo{
			Id:          cfg.Id,
			Revision:    cfg.Revision,
			Images:      cfg.Images,
			CreatedAt:   cfg.CreatedAt,
			UpdatedBy:   cfg.UpdatedBy,
			Reason:      cfg.Reason,
			SpecPreview: cfg.SpecPreview,
			IsCurrent:   cfg.IsCurrent,
		})
	}
	return result
}
