package portalservicelogic

import (
	"context"
	"errors"
	"strings"

	"github.com/yanshicheng/kube-nova/common/handler/errorx"
	"github.com/yanshicheng/kube-nova/common/vars"
	"github.com/yanshicheng/kube-nova/pkg/utils"

	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type RoleSearchLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewRoleSearchLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RoleSearchLogic {
	return &RoleSearchLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// RoleSearch 搜索角色列表
func (l *RoleSearchLogic) RoleSearch(in *pb.SearchSysRoleReq) (*pb.SearchSysRoleResp, error) {
	// 设置默认参数
	if in.Page <= 0 {
		in.Page = vars.Page
	}
	if in.PageSize <= 0 {
		in.PageSize = vars.PageSize
	}
	if in.OrderField == "" {
		in.OrderField = vars.OrderField
	}

	// 构建查询条件
	var conditions []string
	var args []interface{}

	// 角色名称模糊查询
	if in.Name != "" {
		conditions = append(conditions, "`name` LIKE ? AND")
		args = append(args, "%"+strings.TrimSpace(in.Name)+"%")
	}

	// 角色编码模糊查询
	if in.Code != "" {
		conditions = append(conditions, "`code` LIKE ? AND")
		args = append(args, "%"+strings.ToLower(strings.TrimSpace(in.Code))+"%")
	}

	// 去掉最后一个 " AND "，避免 SQL 语法错误
	query := utils.RemoveQueryADN(conditions)

	// 执行分页查询
	roleList, total, err := l.svcCtx.SysRole.Search(l.ctx, in.OrderField, in.IsAsc, in.Page, in.PageSize, query, args...)
	if err != nil && !errors.Is(err, model.ErrNotFound) {
		l.Errorf("查询角色列表失败: %v", err)
		return nil, errorx.Msg("查询角色列表失败")
	}

	// 转换结果
	pbRoles := l.convertToSearchResult(roleList)

	return &pb.SearchSysRoleResp{
		Data:  pbRoles,
		Total: total,
	}, nil
}

// convertToSearchResult 转换搜索结果
func (l *RoleSearchLogic) convertToSearchResult(roleList []*model.SysRole) []*pb.SysRole {
	var pbRoles []*pb.SysRole
	for _, role := range roleList {
		pbRole := &pb.SysRole{
			Id:        role.Id,
			Name:      role.Name,
			Code:      role.Code,
			Remark:    role.Remark,
			CreatedBy: role.CreatedBy,
			UpdatedBy: role.UpdatedBy,
			CreatedAt: role.CreatedAt.Unix(),
			UpdatedAt: role.UpdatedAt.Unix(),
		}
		pbRoles = append(pbRoles, pbRole)
	}
	return pbRoles
}
