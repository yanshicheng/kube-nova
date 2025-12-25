package project

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/manager-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/zeromicro/go-zero/core/logx"
)

type SearchProjectLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 搜索项目列表，支持分页和条件筛选
func NewSearchProjectLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SearchProjectLogic {
	return &SearchProjectLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SearchProjectLogic) SearchProject(req *types.SearchProjectRequest) (resp *types.SearchProjectResponse, err error) {
	// 设置默认分页参数
	if req.Page == 0 {
		req.Page = 1
	}
	if req.PageSize == 0 {
		req.PageSize = 10
	}

	// 调用RPC服务搜索项目
	result, err := l.svcCtx.ManagerRpc.ProjectSearch(l.ctx, &pb.SearchOnecProjectReq{
		Page:     req.Page,
		PageSize: req.PageSize,
		Name:     req.Name,
		Uuid:     req.Uuid,
	})

	if err != nil {
		l.Errorf("搜索项目失败: %v", err)
		return nil, fmt.Errorf("搜索项目失败")
	}

	// 转换数据结构
	var projects []types.Project
	for _, item := range result.Data {
		projects = append(projects, types.Project{
			Id:            item.Id,
			Name:          item.Name,
			Uuid:          item.Uuid,
			Description:   item.Description,
			IsSystem:      item.IsSystem,
			CreatedBy:     item.CreatedBy,
			UpdatedBy:     item.UpdatedBy,
			CreatedAt:     item.CreatedAt,
			UpdatedAt:     item.UpdatedAt,
			AdminCount:    item.AdminCount,
			ResourceCount: item.ResourceCount,
		})
	}

	resp = &types.SearchProjectResponse{
		Items: projects,
		Total: result.Total,
	}

	return resp, nil
}
