package application

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/client/managerservice"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type ApplicationDetailLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取应用详情
func NewApplicationDetailLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ApplicationDetailLogic {
	return &ApplicationDetailLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ApplicationDetailLogic) ApplicationDetail(req *types.DefaultIdRequest) (resp *types.OnecProjectApplication, err error) {
	detail, err := l.svcCtx.ManagerRpc.ApplicationGetById(l.ctx, &managerservice.GetOnecProjectApplicationByIdReq{
		Id: req.Id,
	})
	if err != nil {
		l.Errorf("获取应用详情失败: %v", err)
		return nil, err
	}
	resp = &types.OnecProjectApplication{
		Id:           detail.Data.Id,
		NameCn:       detail.Data.NameCn,
		NameEn:       detail.Data.NameEn,
		ResourceType: detail.Data.ResourceType,
		Description:  detail.Data.Description,
		CreatedAt:    detail.Data.CreatedAt,
		CreatedBy:    detail.Data.CreatedBy,
		UpdatedAt:    detail.Data.UpdatedAt,
		UpdatedBy:    detail.Data.UpdatedBy,
		WorkspaceId:  detail.Data.WorkspaceId,
	}
	return resp, nil
}
