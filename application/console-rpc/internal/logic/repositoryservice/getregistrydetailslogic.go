package repositoryservicelogic

import (
	"context"
	"errors"

	"github.com/yanshicheng/kube-nova/application/console-rpc/internal/model/repository"
	"github.com/yanshicheng/kube-nova/application/console-rpc/internal/svc"
	pb "github.com/yanshicheng/kube-nova/application/console-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetRegistryDetailsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetRegistryDetailsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetRegistryDetailsLogic {
	return &GetRegistryDetailsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetRegistryDetailsLogic) GetRegistryDetails(in *pb.GetRegistryDetailsReq) (*pb.GetRegistryDetailsResp, error) {
	data, err := l.svcCtx.ContainerRegistryModel.FindOneByUuid(l.ctx, in.Uuid)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, errorx.Msg("镜像仓库不存在")
		}
		return nil, errorx.Msg("查询镜像仓库失败")
	}

	resp := &pb.ContainerRegistryDetails{
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
	}

	// 如果是 Harbor 类型，获取统计信息
	if data.Type == "harbor" {
		client, err := l.svcCtx.HarborManager.Get(in.Uuid)
		if err == nil {
			if systemInfo, err := client.SystemInfo(); err == nil {
				resp.StorageTotal = systemInfo.StorageTotal
				resp.StorageUsed = systemInfo.StorageUsed
				resp.StorageFree = systemInfo.StorageFree
				resp.TotalProjects = systemInfo.TotalProjects
				resp.PublicProjects = systemInfo.PublicProjects
				resp.PrivateProjects = systemInfo.PrivateProjects
				resp.TotalRepositories = systemInfo.TotalRepositories
				resp.PublicRepositories = systemInfo.PublicRepositories
				resp.PrivateRepositories = systemInfo.PrivateRepositories
			}
		}
	}

	return &pb.GetRegistryDetailsResp{Data: resp}, nil
}
