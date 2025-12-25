package repositoryservicelogic

import (
	"context"
	"errors"

	"github.com/yanshicheng/kube-nova/application/console-rpc/internal/model/repository"
	"github.com/yanshicheng/kube-nova/application/console-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type DeleteRegistryLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewDeleteRegistryLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteRegistryLogic {
	return &DeleteRegistryLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *DeleteRegistryLogic) DeleteRegistry(in *pb.DeleteRegistryReq) (*pb.DeleteRegistryResp, error) {
	// 软删除
	err := l.svcCtx.ContainerRegistryModel.DeleteSoft(l.ctx, in.Id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, errorx.Msg("镜像仓库不存在")
		}
		return nil, errorx.Msg("删除镜像仓库失败")
	}

	return &pb.DeleteRegistryResp{Message: "删除成功"}, nil
}
