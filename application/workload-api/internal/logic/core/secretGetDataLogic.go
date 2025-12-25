package core

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/client/managerservice"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type SecretGetDataLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取 Secret 数据（key-value 列表）
func NewSecretGetDataLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SecretGetDataLogic {
	return &SecretGetDataLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SecretGetDataLogic) SecretGetData(req *types.DefaultNameRequest) (resp *types.GetSecretDataResponse, err error) {
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

	secretData, err := secretClient.GetData(workloadInfo.Data.Namespace, req.Name)
	if err != nil {
		l.Errorf("获取 Secret 数据失败: %v", err)
		return nil, fmt.Errorf("获取 Secret 数据失败")
	}

	items := make([]types.SecretDataItem, 0, len(secretData.Data))
	for _, item := range secretData.Data {
		items = append(items, types.SecretDataItem{
			Key:   item.Key,
			Value: item.Value,
		})
	}

	l.Infof("成功获取 Secret 数据: %s, 共 %d 项", req.Name, len(items))
	return &types.GetSecretDataResponse{
		Name:      secretData.Name,
		Namespace: secretData.Namespace,
		Type:      secretData.Type,
		Data:      items,
	}, nil
}
