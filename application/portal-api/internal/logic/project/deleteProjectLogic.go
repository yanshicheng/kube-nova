package project

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/portal-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type DeleteProjectLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewDeleteProjectLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteProjectLogic {
	return &DeleteProjectLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DeleteProjectLogic) DeleteProject(req *types.PortalProjectIdReq) error {
	_, err := l.svcCtx.ProjectRpc.DeleteProject(l.ctx, &pb.PortalDeleteProjectReq{
		Id:        req.Id,
		UpdatedBy: currentUsername(l.ctx),
	})
	if err != nil {
		l.Errorf("删除项目失败: %v", err)
		return err
	}

	return nil
}
