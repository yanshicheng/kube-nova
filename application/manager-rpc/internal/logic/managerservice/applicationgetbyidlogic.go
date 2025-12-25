package managerservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type ApplicationGetByIdLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewApplicationGetByIdLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ApplicationGetByIdLogic {
	return &ApplicationGetByIdLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ApplicationGetByIdLogic) ApplicationGetById(in *pb.GetOnecProjectApplicationByIdReq) (*pb.GetOnecProjectApplicationByIdResp, error) {
	app, err := l.svcCtx.OnecProjectApplication.FindOne(l.ctx, in.Id)
	if err != nil {
		l.Errorf("查询应用失败: %v, id=%d", err, in.Id)
		return nil, errorx.Msg("查询应用失败")
	}
	return &pb.GetOnecProjectApplicationByIdResp{
		Data: &pb.OnecProjectApplication{
			Id:           app.Id,
			NameCn:       app.NameCn,
			NameEn:       app.NameEn,
			WorkspaceId:  app.WorkspaceId,
			ResourceType: app.ResourceType,
			Description:  app.Description,
			CreatedAt:    app.CreatedAt.Unix(),
			UpdatedAt:    app.UpdatedAt.Unix(),
			CreatedBy:    app.CreatedBy,
			UpdatedBy:    app.UpdatedBy,
		},
	}, nil
}
