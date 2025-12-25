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

type BindRegistryToClusterLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewBindRegistryToClusterLogic(ctx context.Context, svcCtx *svc.ServiceContext) *BindRegistryToClusterLogic {
	return &BindRegistryToClusterLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ============ 仓库与集群关联（管理员操作）============
func (l *BindRegistryToClusterLogic) BindRegistryToCluster(in *pb.BindRegistryToClusterReq) (*pb.BindRegistryToClusterResp, error) {
	// 检查仓库是否存在
	_, err := l.svcCtx.ContainerRegistryModel.FindOne(l.ctx, in.RegistryId)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, errorx.Msg("镜像仓库不存在")
		}
		return nil, errorx.Msg("查询镜像仓库失败")
	}

	// 检查是否已绑定
	existing, _ := l.svcCtx.RepositoryClusterModel.FindOneByRegistryIdClusterUuidIsDeleted(
		l.ctx,
		in.RegistryId,
		in.ClusterUuid,
		0,
	)
	if existing != nil {
		return nil, errorx.Msg("该仓库已绑定到此集群")
	}

	// 创建绑定
	data := &repository.RegistryCluster{
		RegistryId:  in.RegistryId,
		ClusterUuid: in.ClusterUuid,
		IsDeleted:   0,
	}

	result, err := l.svcCtx.RepositoryClusterModel.Insert(l.ctx, data)
	if err != nil {
		return nil, errorx.Msg("绑定仓库到集群失败")
	}

	id, _ := result.LastInsertId()
	return &pb.BindRegistryToClusterResp{
		Id:      uint64(id),
		Message: "绑定成功",
	}, nil
}
