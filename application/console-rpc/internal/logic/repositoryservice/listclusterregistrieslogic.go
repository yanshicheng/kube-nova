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

type ListClusterRegistriesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListClusterRegistriesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListClusterRegistriesLogic {
	return &ListClusterRegistriesLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ListClusterRegistriesLogic) ListClusterRegistries(in *pb.ListClusterRegistriesReq) (*pb.ListClusterRegistriesResp, error) {
	var queryStr string
	var args []interface{}

	if in.RegistryId != 0 {
		queryStr = "`cluster_uuid` = ? AND `registry_id` = ?"
		args = []interface{}{in.ClusterUuid, in.RegistryId}
	} else {
		queryStr = "`cluster_uuid` = ?"
		args = []interface{}{in.ClusterUuid}
	}

	list, err := l.svcCtx.RepositoryClusterModel.SearchNoPage(l.ctx, "", true, queryStr, args...)
	if err != nil && !errors.Is(err, repository.ErrNotFound) {
		return nil, errorx.Msg("查询集群仓库关联失败")
	}

	var data []*pb.RegistryCluster
	for _, item := range list {
		// 查询关联的仓库信息
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
				Id:          registry.Id,
				Name:        registry.Name,
				Uuid:        registry.Uuid,
				Type:        registry.Type,
				Env:         registry.Env,
				Url:         registry.Url,
				Username:    registry.Username,
				Password:    registry.Password,
				Insecure:    registry.Insecure == 1,
				CaCert:      registry.CaCert.String,
				Config:      registry.Config.String,
				Status:      int32(registry.Status),
				Description: registry.Description,
				CreatedBy:   registry.CreatedBy,
				UpdatedBy:   registry.UpdatedBy,
				CreatedAt:   registry.CreatedAt.Unix(),
				UpdatedAt:   registry.UpdatedAt.Unix(),
			}
		}

		data = append(data, pbItem)
	}

	return &pb.ListClusterRegistriesResp{Data: data}, nil
}
