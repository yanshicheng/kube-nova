// roleBindingGetAssociationLogic.go
package core

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type RoleBindingGetAssociationLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewRoleBindingGetAssociationLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RoleBindingGetAssociationLogic {
	return &RoleBindingGetAssociationLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *RoleBindingGetAssociationLogic) RoleBindingGetAssociation(req *types.ClusterNamespaceResourceRequest) (resp *types.RoleBindingAssociation, err error) {
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

	// 获取 RoleBinding operator
	rbOp := client.RoleBindings()

	// 获取关联信息
	association, err := rbOp.GetAssociation(req.Namespace, req.Name)
	if err != nil {
		l.Errorf("获取 RoleBinding 关联信息失败: %v", err)
		return nil, fmt.Errorf("获取 RoleBinding 关联信息失败")
	}

	// 转换 Subjects
	subjects := make([]types.SubjectInfo, 0, len(association.Subjects))
	for _, subj := range association.Subjects {
		subjects = append(subjects, types.SubjectInfo{
			Kind:      subj.Kind,
			Name:      subj.Name,
			Namespace: subj.Namespace,
			APIGroup:  subj.APIGroup,
		})
	}

	resp = &types.RoleBindingAssociation{
		RoleBindingName: association.RoleBindingName,
		Namespace:       association.Namespace,
		RoleName:        association.RoleName,
		RoleKind:        association.RoleKind,
		Subjects:        subjects,
		SubjectCount:    len(subjects),
	}

	l.Infof("用户: %s, 成功获取 RoleBinding 关联信息: %s/%s", username, req.Namespace, req.Name)
	return resp, nil
}
