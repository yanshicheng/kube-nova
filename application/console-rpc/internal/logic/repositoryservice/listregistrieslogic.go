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

type ListRegistriesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListRegistriesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListRegistriesLogic {
	return &ListRegistriesLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ============ 仓库查询（管理员视角 + 项目视角）============
func (l *ListRegistriesLogic) ListRegistries(in *pb.ListRegistriesReq) (*pb.ListRegistriesResp, error) {
	// 构建查询条件
	var conditions []string
	var args []interface{}

	if in.Name != "" {
		conditions = append(conditions, "`name` LIKE ?")
		args = append(args, "%"+in.Name+"%")
	}
	if in.Uuid != "" {
		conditions = append(conditions, "`uuid` = ?")
		args = append(args, in.Uuid)
	}
	if in.Env != "" {
		conditions = append(conditions, "`env` = ?")
		args = append(args, in.Env)
	}
	if in.Type != "" {
		conditions = append(conditions, "`type` = ?")
		args = append(args, in.Type)
	}
	if in.Status != 0 {
		conditions = append(conditions, "`status` = ?")
		args = append(args, in.Status)
	}

	queryStr := ""
	if len(conditions) > 0 {
		queryStr = strings.Join(conditions, " AND ")
	}

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
		return nil, errorx.Msg("查询镜像仓库列表失败")
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

	return &pb.ListRegistriesResp{
		Data:  data,
		Total: total,
	}, nil
}
