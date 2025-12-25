package application

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/client/managerservice"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type ApplicationSearchLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 查询应用列表
func NewApplicationSearchLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ApplicationSearchLogic {
	return &ApplicationSearchLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ApplicationSearchLogic) ApplicationSearch(req *types.SearchOnecProjectApplicationReq) (resp []types.OnecProjectApplication, err error) {
	searchResp, err := l.svcCtx.ManagerRpc.ApplicationSearch(l.ctx, &managerservice.SearchOnecProjectApplicationReq{
		WorkspaceId: req.WorkspaceId,
		NameCn:      req.NameCn,
	})
	if err != nil {
		l.Errorf("查询应用列表失败: %v", err)
		return nil, err
	}

	var applications []types.OnecProjectApplication
	for _, app := range searchResp.Data {
		applications = append(applications, types.OnecProjectApplication{
			Id:           app.Id,
			WorkspaceId:  app.WorkspaceId,
			NameCn:       app.NameCn,
			NameEn:       app.NameEn,
			ResourceType: app.ResourceType,
			Description:  app.Description,
			CreatedBy:    app.CreatedBy,
			UpdatedBy:    app.UpdatedBy,
			CreatedAt:    app.CreatedAt,
			UpdatedAt:    app.UpdatedAt,
		})
	}

	return applications, nil
}
