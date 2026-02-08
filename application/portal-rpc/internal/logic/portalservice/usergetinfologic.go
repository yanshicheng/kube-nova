package portalservicelogic

import (
	"context"
	"fmt"
	"strings"

	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type UserGetInfoLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUserGetInfoLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UserGetInfoLogic {
	return &UserGetInfoLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 用户个人详情
func (l *UserGetInfoLogic) UserGetInfo(in *pb.GetSysUserInfoReq) (*pb.GetSysUserInfoResp, error) {
	// 参数验证
	if in.Id == 0 {
		l.Error("用户ID不能为空")
		return nil, errorx.Msg("用户ID不能为空")
	}

	// 查询用户基本信息
	user, err := l.svcCtx.SysUser.FindOne(l.ctx, in.Id)
	if err != nil {
		l.Errorf("查询用户失败，用户ID: %d, 错误: %v", in.Id, err)
		return nil, errorx.Msg("用户不存在")
	}

	// 查询用户部门信息并构建部门路径
	var deptNames string
	if user.DeptId > 0 {
		deptNames = l.buildDeptPath(user.DeptId)
	}

	// 查询用户角色信息
	roleNames := l.getUserRoleNames(in.Id)

	absIcon := fmt.Sprintf("%s/%s%s", l.svcCtx.Config.StorageConf.EndpointProxy, l.svcCtx.Config.StorageConf.BucketName, user.Avatar)
	// 构建返回数据
	resp := &pb.GetSysUserInfoResp{
		Id:             user.Id,
		Username:       user.Username,
		Nickname:       user.Nickname,
		Avatar:         absIcon,
		Email:          user.Email,
		Phone:          user.Phone,
		WorkNumber:     user.WorkNumber,
		Status:         user.Status,
		IsNeedResetPwd: user.IsNeedResetPwd,
		CreatedBy:      user.CreatedBy,
		UpdatedBy:      user.UpdatedBy,
		CreatedAt:      user.CreatedAt.Unix(),
		UpdatedAt:      user.UpdatedAt.Unix(),
		DeptNames:      deptNames,
		RoleNames:      roleNames,
		DingtalkId:     user.DingtalkId.String,
		WechatId:       user.WechatId.String,
		FeishuId:       user.FeishuId.String,
	}

	return resp, nil
}

// buildDeptPath 构建部门路径
func (l *UserGetInfoLogic) buildDeptPath(deptId uint64) string {
	var deptNames []string
	currentDeptId := deptId

	// 向上查找所有父级部门
	for currentDeptId > 0 {
		dept, err := l.svcCtx.SysDept.FindOne(l.ctx, currentDeptId)
		if err != nil {
			break
		}
		deptNames = append([]string{dept.Name}, deptNames...) // 前置插入
		currentDeptId = dept.ParentId
	}

	return strings.Join(deptNames, "/")
}

// getUserRoleNames 获取用户角色名称
func (l *UserGetInfoLogic) getUserRoleNames(userId uint64) string {
	// 查询用户角色关联
	userRoles, err := l.svcCtx.SysUserRole.SearchNoPage(l.ctx, "", true, "user_id = ?", userId)
	if err != nil {
		return ""
	}

	// 查询角色名称
	var roleNames []string
	for _, userRole := range userRoles {
		role, err := l.svcCtx.SysRole.FindOne(l.ctx, userRole.RoleId)
		if err != nil {
			continue
		}
		roleNames = append(roleNames, role.Name)
	}

	return strings.Join(roleNames, ",")
}
