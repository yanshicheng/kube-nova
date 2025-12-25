// pVCGetAssociationLogic.go
package core

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type PVCGetAssociationLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewPVCGetAssociationLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PVCGetAssociationLogic {
	return &PVCGetAssociationLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *PVCGetAssociationLogic) PVCGetAssociation(req *types.ClusterNamespaceResourceRequest) (resp *types.PVCAssociation, err error) {
	username, ok := l.ctx.Value("username").(string)
	if !ok {
		username = "system"
	}

	// 获取集群客户端
	client, err := l.svcCtx.K8sManager.GetCluster(l.ctx, req.ClusterUuid)
	if err != nil {
		l.Errorf("获取集群客户端失败: %v", err)
		return nil, fmt.Errorf("获取集群客户端失败")
	}

	// 获取 PVC operator
	pvcOp := client.PVC()

	// 获取关联信息
	association, err := pvcOp.GetAssociation(req.Namespace, req.Name)
	if err != nil {
		l.Errorf("获取 PVC 关联信息失败: %v", err)
		return nil, fmt.Errorf("获取 PVC 关联信息失败")
	}
	// 转换响应格式
	resp = &types.PVCAssociation{
		PVCName:   association.PVCName,
		Namespace: association.Namespace,
		PVName:    association.PVName,
		Pods:      association.Pods,
		PodCount:  association.PodCount,
	}

	l.Infof("用户: %s, 成功获取 PVC 关联信息: %s/%s", username, req.Namespace, req.Name)
	return resp, nil
}
