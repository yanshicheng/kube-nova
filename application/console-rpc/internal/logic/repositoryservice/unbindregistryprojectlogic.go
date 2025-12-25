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

type UnbindRegistryProjectLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUnbindRegistryProjectLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UnbindRegistryProjectLogic {
	return &UnbindRegistryProjectLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *UnbindRegistryProjectLogic) UnbindRegistryProject(in *pb.UnbindRegistryProjectReq) (*pb.UnbindRegistryProjectResp, error) {
	err := l.svcCtx.RegistryProjectBindingModel.Delete(l.ctx, in.Id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, errorx.Msg("绑定关系不存在")
		}
		return nil, errorx.Msg("解绑失败")
	}

	return &pb.UnbindRegistryProjectResp{Message: "解绑成功"}, nil
}
