package repositoryservicelogic

import (
	"context"
	"errors"
	"strings"

	"github.com/yanshicheng/kube-nova/application/console-rpc/internal/model/repository"
	"github.com/yanshicheng/kube-nova/application/console-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListRegistriesByProjectLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListRegistriesByProjectLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListRegistriesByProjectLogic {
	return &ListRegistriesByProjectLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 项目视角：查询项目关联的仓库
func (l *ListRegistriesByProjectLogic) ListRegistriesByProject(in *pb.ListRegistriesByProjectReq) (*pb.ListRegistriesByProjectResp, error) {
	// 查询项目关联的 registry_cluster
	bindings, err := l.svcCtx.RepositoryClusterModel.SearchNoPage(
		l.ctx,
		"",
		true,
		"`cluster_uuid` = ?",
		in.ClusterUuid,
	)
	if err != nil && !errors.Is(err, repository.ErrNotFound) {
		return nil, errorx.Msg("查询项目关联仓库失败")
	}

	if len(bindings) == 0 {
		return &pb.ListRegistriesByProjectResp{Data: []*pb.ContainerRegistry{}, Total: 0}, nil
	}

	// 获取所有 registry_id
	var registryIds []interface{}
	for _, binding := range bindings {
		registryIds = append(registryIds, binding.RegistryId)
	}

	// 构建查询条件
	var conditions []string
	var args []interface{}

	// 添加 registry_id IN 条件
	placeholders := make([]string, len(registryIds))
	for i := range placeholders {
		placeholders[i] = "?"
	}
	conditions = append(conditions, "`id` IN ("+strings.Join(placeholders, ",")+")")
	args = append(args, registryIds...)

	if in.Name != "" {
		conditions = append(conditions, "`name` LIKE ?")
		args = append(args, "%"+in.Name+"%")
	}
	if in.Env != "" {
		conditions = append(conditions, "`env` = ?")
		args = append(args, in.Env)
	}
	if in.Type != "" {
		conditions = append(conditions, "`type` = ?")
		args = append(args, in.Type)
	}

	queryStr := strings.Join(conditions, " AND ")

	// 查询数据
	list, total, err := l.svcCtx.ContainerRegistryModel.Search(
		l.ctx,
		in.OrderBy,
		in.IsAsc,
		in.Page,
		in.PageSize,
		queryStr,
		args...,
	)
	if err != nil && !errors.Is(err, repository.ErrNotFound) {
		return nil, errorx.Msg("查询仓库列表失败")
	}

	// 转换数据
	var data []*pb.ContainerRegistry
	for _, item := range list {
		registry := &pb.ContainerRegistry{
			Id:          item.Id,
			Name:        item.Name,
			Uuid:        item.Uuid,
			Type:        item.Type,
			Env:         item.Env,
			Url:         item.Url,
			Username:    item.Username,
			Password:    item.Password,
			Insecure:    item.Insecure == 1,
			CaCert:      item.CaCert.String,
			Config:      item.Config.String,
			Status:      int32(item.Status),
			Description: item.Description,
			CreatedBy:   item.CreatedBy,
			UpdatedBy:   item.UpdatedBy,
			CreatedAt:   item.CreatedAt.Unix(),
			UpdatedAt:   item.UpdatedAt.Unix(),
		}
		if item.Type == "harbor" {
			client, err := l.svcCtx.HarborManager.Get(item.Uuid)
			if err == nil {
				if systemInfo, err := client.SystemInfo(); err == nil {
					registry.TotalProjects = systemInfo.TotalProjects
					registry.TotalRepositories = systemInfo.TotalRepositories
					registry.StorageTotal = systemInfo.StorageTotal
				}
			}
		}
		data = append(data, registry)
	}

	return &pb.ListRegistriesByProjectResp{
		Data:  data,
		Total: total,
	}, nil
}
