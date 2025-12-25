package core

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/client/managerservice"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type ListSecretsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取 Secret 列表
func NewListSecretsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListSecretsLogic {
	return &ListSecretsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListSecretsLogic) ListSecrets(req *types.SecretListRequest) (resp *types.SecretListResponse, err error) {
	workloadInfo, err := l.svcCtx.ManagerRpc.ProjectWorkspaceGetById(l.ctx, &managerservice.GetOnecProjectWorkspaceByIdReq{Id: req.WorkloadId})
	if err != nil {
		l.Errorf("获取项目工作空间详情失败: %v", err)
		return nil, fmt.Errorf("获取项目工作空间详情失败")
	}

	client, err := l.svcCtx.K8sManager.GetCluster(l.ctx, workloadInfo.Data.ClusterUuid)
	if err != nil {
		l.Errorf("获取集群客户端失败: %v", err)
		return nil, fmt.Errorf("获取集群客户端失败")
	}

	secretClient := client.Secrets()

	secretList, err := secretClient.List(
		workloadInfo.Data.Namespace,
		req.Search,
		req.LabelSelector,
		req.Type, // 传递类型过滤参数
	)
	if err != nil {
		l.Errorf("获取 Secret 列表失败: %v", err)
		return nil, fmt.Errorf("获取 Secret 列表失败")
	}

	// 5. 转换响应格式
	items := make([]types.SecretListItem, 0, len(secretList.Items))
	for _, secret := range secretList.Items {
		items = append(items, types.SecretListItem{
			Name:              secret.Name,
			Namespace:         secret.Namespace,
			Type:              secret.Type,
			DataCount:         secret.DataCount,
			Age:               secret.Age,
			CreationTimestamp: secret.CreationTimestamp,
			Labels:            secret.Labels,
		})
	}

	// 记录日志
	if req.Type != "" {
		l.Infof("成功获取 Secret 列表（类型: %s），共 %d 个", req.Type, len(items))
	} else {
		l.Infof("成功获取 Secret 列表，共 %d 个", len(items))
	}

	return &types.SecretListResponse{
		Total: secretList.Total,
		Items: items,
	}, nil
}
