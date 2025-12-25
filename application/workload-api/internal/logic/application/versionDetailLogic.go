package application

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/client/managerservice"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type VersionDetailLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 查询版本详情
func NewVersionDetailLogic(ctx context.Context, svcCtx *svc.ServiceContext) *VersionDetailLogic {
	return &VersionDetailLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *VersionDetailLogic) VersionDetail(req *types.DefaultIdRequest) (resp *types.OnecProjectVersion, err error) {
	detail, err := l.svcCtx.ManagerRpc.VersionDetail(l.ctx, &managerservice.GetOnecProjectVersionDetailReq{
		Id: req.Id,
	})
	if err != nil {
		l.Errorf("获取版本详情失败: %v", err)
		return nil, fmt.Errorf("获取版本详情失败")
	}
	resp = &types.OnecProjectVersion{
		Id:            detail.VersionId,
		ApplicationId: detail.ApplicationId,
		ResourceName:  detail.ResourceName,
		Version:       detail.Version,
	}
	return
}
