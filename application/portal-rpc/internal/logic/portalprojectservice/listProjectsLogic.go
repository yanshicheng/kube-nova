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
	if in.Id > 0 {
		conditions = append(conditions, "`id` = ?")
		args = append(args, in.Id)
	}
	if in.Uuid != "" {
		conditions = append(conditions, "`uuid` = ?")
		args = append(args, in.Uuid)
	}
	if in.IsSystem >= 0 {
		conditions = append(conditions, "`is_system` = ?")
		args = append(args, in.IsSystem)
	}
	if in.UserId > 0 && in.PlatformId > 0 {
		conditions = append(conditions, "`id` in (select pmpr.`project_id` from `project_member_platform_role` pmpr inner join `project_platform_binding` ppb on ppb.`project_id` = pmpr.`project_id` and ppb.`platform_id` = pmpr.`platform_id` and ppb.`is_deleted` = 0 where pmpr.`user_id` = ? and pmpr.`platform_id` = ? and pmpr.`is_deleted` = 0)")
		args = append(args, in.UserId, in.PlatformId)
	} else if in.UserId > 0 {
		conditions = append(conditions, "`id` in (select `project_id` from `project_member_binding` where `user_id` = ? and `is_deleted` = 0)")
		args = append(args, in.UserId)
	}
	if in.PlatformId > 0 && in.UserId == 0 {
		conditions = append(conditions, "`id` in (select `project_id` from `project_platform_binding` where `platform_id` = ? and `is_deleted` = 0)")
		args = append(args, in.PlatformId)
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

	return &pb.PortalListProjectsResp{
		Data:  projectsToPb(projects),
		Total: total,
	}, nil
}
