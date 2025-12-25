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

type GetRegistryByUuidLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetRegistryByUuidLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetRegistryByUuidLogic {
	return &GetRegistryByUuidLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetRegistryByUuidLogic) GetRegistryByUuid(in *pb.GetRegistryByUuidReq) (*pb.GetRegistryByUuidResp, error) {
	data, err := l.svcCtx.ContainerRegistryModel.FindOneByUuid(l.ctx, in.Uuid)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, errorx.Msg("镜像仓库不存在")
		}
		return nil, errorx.Msg("查询镜像仓库失败")
	}

	return &pb.GetRegistryByUuidResp{
		Data: &pb.ContainerRegistry{
			Id:          data.Id,
			Name:        data.Name,
			Uuid:        data.Uuid,
			Type:        data.Type,
			Env:         data.Env,
			Url:         data.Url,
			Username:    data.Username,
			Password:    data.Password,
			Insecure:    data.Insecure == 1,
			CaCert:      data.CaCert.String,
			Config:      data.Config.String,
			Status:      int32(data.Status),
			Description: data.Description,
			CreatedBy:   data.CreatedBy,
			UpdatedBy:   data.UpdatedBy,
			CreatedAt:   data.CreatedAt.Unix(),
			UpdatedAt:   data.UpdatedAt.Unix(),
		},
	}, nil
}
