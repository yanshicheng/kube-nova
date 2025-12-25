package repositoryservicelogic

import (
	"context"
	"database/sql"
	"errors"

	"github.com/yanshicheng/kube-nova/application/console-rpc/internal/model/repository"
	"github.com/yanshicheng/kube-nova/application/console-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type UpdateRegistryLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpdateRegistryLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateRegistryLogic {
	return &UpdateRegistryLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *UpdateRegistryLogic) UpdateRegistry(in *pb.UpdateRegistryReq) (*pb.UpdateRegistryResp, error) {
	// 查询原数据
	oldData, err := l.svcCtx.ContainerRegistryModel.FindOne(l.ctx, in.Id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, errorx.Msg("镜像仓库不存在")
		}
		return nil, errorx.Msg("查询镜像仓库失败")
	}

	// 更新数据
	oldData.Name = in.Name
	oldData.Type = in.Type
	oldData.Env = in.Env
	oldData.Url = in.Url
	oldData.Username = in.Username
	oldData.Password = in.Password
	oldData.Insecure = 0
	if in.Insecure {
		oldData.Insecure = 1
	}
	oldData.Status = int64(in.Status)
	oldData.Description = in.Description
	oldData.UpdatedBy = in.UpdatedBy

	if in.CaCert != "" {
		oldData.CaCert = sql.NullString{String: in.CaCert, Valid: true}
	}
	if in.Config != "" {
		oldData.Config = sql.NullString{String: in.Config, Valid: true}
	}

	err = l.svcCtx.ContainerRegistryModel.Update(l.ctx, oldData)
	if err != nil {
		return nil, errorx.Msg("更新镜像仓库失败")
	}

	return &pb.UpdateRegistryResp{Message: "更新成功"}, nil
}
