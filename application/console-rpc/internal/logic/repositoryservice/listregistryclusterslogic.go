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

type ListRegistryClustersLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListRegistryClustersLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListRegistryClustersLogic {
	return &ListRegistryClustersLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ListRegistryClustersLogic) ListRegistryClusters(in *pb.ListRegistryClustersReq) (*pb.ListRegistryClustersResp, error) {
	list, err := l.svcCtx.RepositoryClusterModel.SearchNoPage(
		l.ctx,
		"",
		true,
		"`registry_id` = ?",
		in.RegistryId,
	)
	if err != nil && !errors.Is(err, repository.ErrNotFound) {
		return nil, errorx.Msg("查询仓库集群关联失败")
	}

	var data []*pb.RegistryCluster
	for _, item := range list {
		registry, _ := l.svcCtx.ContainerRegistryModel.FindOne(l.ctx, item.RegistryId)

		pbItem := &pb.RegistryCluster{
			Id:          item.Id,
			RegistryId:  item.RegistryId,
			ClusterUuid: item.ClusterUuid,
			CreatedAt:   item.CreatedAt.Unix(),
			UpdatedAt:   item.UpdatedAt.Unix(),
		}

		if registry != nil {
			pbItem.Registry = &pb.ContainerRegistry{
				Id:       registry.Id,
				Name:     registry.Name,
				Uuid:     registry.Uuid,
				Type:     registry.Type,
				Env:      registry.Env,
				Url:      registry.Url,
				Username: registry.Username,
				Password: registry.Password,
				Insecure: registry.Insecure == 1,
			}
		}

		data = append(data, pbItem)
	}

	return &pb.ListRegistryClustersResp{Data: data}, nil
}
