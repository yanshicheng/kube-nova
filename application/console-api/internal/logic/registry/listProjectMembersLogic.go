package registry

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/console-rpc/pb"
	"github.com/zeromicro/go-zero/core/logx"
)

type ListProjectMembersLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 列出项目成员
func NewListProjectMembersLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListProjectMembersLogic {
	return &ListProjectMembersLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListProjectMembersLogic) ListProjectMembers(req *types.ListProjectMembersRequest) (resp *types.ListProjectMembersResponse, err error) {
	rpcResp, err := l.svcCtx.RepositoryRpc.ListProjectMembers(l.ctx, &pb.ListProjectMembersReq{
		RegistryUuid: req.RegistryUuid,
		ProjectName:  req.ProjectName,
		Search:       req.Search,
		Page:         req.Page,
		PageSize:     req.PageSize,
	})
	if err != nil {
		l.Errorf("RPC调用失败: %v", err)
		return nil, err
	}

	var items []types.ProjectMember
	for _, item := range rpcResp.Items {
		items = append(items, types.ProjectMember{
			Id:           item.Id,
			ProjectId:    item.ProjectId,
			EntityName:   item.EntityName,
			EntityType:   item.EntityType,
			RoleId:       item.RoleId,
			RoleName:     item.RoleName,
			CreationTime: item.CreationTime,
			UpdateTime:   item.UpdateTime,
		})
	}

	l.Infof("查询项目成员成功: Total=%d", rpcResp.Total)
	return &types.ListProjectMembersResponse{
		Items:      items,
		Total:      rpcResp.Total,
		Page:       rpcResp.Page,
		PageSize:   rpcResp.PageSize,
		TotalPages: rpcResp.TotalPages,
	}, nil
}
