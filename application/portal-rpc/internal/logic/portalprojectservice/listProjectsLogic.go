package portalprojectservicelogic

import (
	"context"
	"fmt"
	"strings"

	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListProjectsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListProjectsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListProjectsLogic {
	return &ListProjectsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ListProjectsLogic) ListProjects(in *pb.PortalListProjectsReq) (*pb.PortalListProjectsResp, error) {
	var conditions []string
	var args []any

	if in.Name != "" {
		conditions = append(conditions, "`name` LIKE ?")
		args = append(args, "%"+in.Name+"%")
	}
	if in.Uuid != "" {
		conditions = append(conditions, "`uuid` = ?")
		args = append(args, in.Uuid)
	}
	if in.IsSystem >= 0 {
		conditions = append(conditions, "`is_system` = ?")
		args = append(args, in.IsSystem)
	}

	queryStr := strings.Join(conditions, " AND ")

	page := in.Page
	if page == 0 {
		page = 1
	}
	pageSize := in.PageSize
	if pageSize == 0 {
		pageSize = 20
	}

	projects, total, err := l.svcCtx.OnecProjectModel.Search(l.ctx, "id", false, page, pageSize, queryStr, args...)
	if err != nil {
		l.Errorf("查询项目列表失败: %v", err)
		return nil, fmt.Errorf("查询项目列表失败: %v", err)
	}

	var data []*pb.PortalProject
	for _, p := range projects {
		data = append(data, &pb.PortalProject{
			Id:          p.Id,
			Name:        p.Name,
			Uuid:        p.Uuid,
			IsSystem:    p.IsSystem,
			Description: p.Description,
			CreatedBy:   p.CreatedBy,
			UpdatedBy:   p.UpdatedBy,
			CreatedAt:   p.CreatedAt.Unix(),
			UpdatedAt:   p.UpdatedAt.Unix(),
		})
	}

	return &pb.PortalListProjectsResp{
		Data:  data,
		Total: total,
	}, nil
}
