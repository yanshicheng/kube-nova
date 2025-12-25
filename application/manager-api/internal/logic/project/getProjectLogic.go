package project

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/manager-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetProjectLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 根据ID获取项目详细信息
func NewGetProjectLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetProjectLogic {
	return &GetProjectLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetProjectLogic) GetProject(req *types.DefaultIdRequest) (resp *types.Project, err error) {
	// 调用RPC服务获取项目详情
	result, err := l.svcCtx.ManagerRpc.ProjectGetById(l.ctx, &pb.GetOnecProjectByIdReq{
		Id: req.Id,
	})

	if err != nil {
		l.Errorf("获取项目详情失败: %v", err)
		return nil, fmt.Errorf("获取项目详情失败: %v", err)
	}

	// 转换数据结构
	resp = &types.Project{
		Id:            result.Data.Id,
		Name:          result.Data.Name,
		Uuid:          result.Data.Uuid,
		Description:   result.Data.Description,
		AdminCount:    result.Data.AdminCount,
		ResourceCount: result.Data.ResourceCount,
		IsSystem:      result.Data.IsSystem,
		CreatedBy:     result.Data.CreatedBy,
		UpdatedBy:     result.Data.UpdatedBy,
		CreatedAt:     result.Data.CreatedAt,
		UpdatedAt:     result.Data.UpdatedAt,
	}

	return resp, nil
}
