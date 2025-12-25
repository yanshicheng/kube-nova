package portalservicelogic

import (
	"context"
	"fmt"
	"strings"

	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"
	"github.com/yanshicheng/kube-nova/common/vars"

	"github.com/zeromicro/go-zero/core/logx"
)

type UserSearchLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUserSearchLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UserSearchLogic {
	return &UserSearchLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// UserSearch 搜索用户列表
func (l *UserSearchLogic) UserSearch(in *pb.SearchSysUserReq) (*pb.SearchSysUserResp, error) {
	// 参数验证
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

	if in.Username != "" {
		conditions = append(conditions, "username LIKE ?")
		args = append(args, "%"+in.Username+"%")
	}
	if in.Nickname != "" {
		conditions = append(conditions, "nickname LIKE ?")
		args = append(args, "%"+in.Nickname+"%")
	}
	if in.Email != "" {
		conditions = append(conditions, "email LIKE ?")
		args = append(args, "%"+in.Email+"%")
	}
	if in.Phone != "" {
		conditions = append(conditions, "phone LIKE ?")
		args = append(args, "%"+in.Phone+"%")
	}
	if in.WorkNumber != "" {
		conditions = append(conditions, "work_number LIKE ?")
		args = append(args, "%"+in.WorkNumber+"%")
	}

	if in.CreatedBy != "" {
		conditions = append(conditions, "created_by LIKE ?")
		args = append(args, "%"+in.CreatedBy+"%")
	}
	if in.UpdatedBy != "" {
		conditions = append(conditions, "updated_by LIKE ?")
		args = append(args, "%"+in.UpdatedBy+"%")
	}
	queryStr := strings.Join(conditions, " AND ")

	// 执行查询
	users, total, err := l.svcCtx.SysUser.Search(l.ctx, in.OrderField, in.IsAsc, in.Page, in.PageSize, queryStr, args...)
	if err != nil {
		l.Errorf("查询用户列表失败，错误: %v", err)
		return nil, errorx.Msg("查询用户列表失败")
	}

	// 转换为protobuf格式
	pbUsers := make([]*pb.SysUser, 0, len(users))
	for _, user := range users {
		absIcon := fmt.Sprintf("%s/%s%s", l.svcCtx.Config.StorageConf.EndpointProxy, l.svcCtx.Config.StorageConf.BucketName, user.Avatar)
		pbUsers = append(pbUsers, &pb.SysUser{
			Id:             user.Id,
			Username:       user.Username,
			Nickname:       user.Nickname,
			Avatar:         absIcon,
			Email:          user.Email,
			Phone:          user.Phone,
			WorkNumber:     user.WorkNumber,
			DeptId:         user.DeptId,
			Status:         user.Status,
			IsNeedResetPwd: user.IsNeedResetPwd,
			CreatedBy:      user.CreatedBy,
			UpdatedBy:      user.UpdatedBy,
			CreatedAt:      user.CreatedAt.Unix(),
			UpdatedAt:      user.UpdatedAt.Unix(),
		})
	}

	return &pb.SearchSysUserResp{
		Data:  pbUsers,
		Total: total,
	}, nil
}
