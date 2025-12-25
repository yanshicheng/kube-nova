package core

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ServiceAccountDescribeLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取 ServiceAccount 详细描述
func NewServiceAccountDescribeLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ServiceAccountDescribeLogic {
	return &ServiceAccountDescribeLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ServiceAccountDescribeLogic) ServiceAccountDescribe(req *types.ClusterNamespaceResourceRequest) (resp string, err error) {
	// todo: add your logic here and delete this line

	return
}
