package managerservicelogic

import (
	"context"
	"strings"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type ProjectGetByUserIdLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewProjectGetByUserIdLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ProjectGetByUserIdLogic {
	return &ProjectGetByUserIdLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ProjectGetByUserIdLogic) ProjectGetByUserId(in *pb.GetOnecProjectsByUserIdReq) (*pb.GetOnecProjectsByUserIdResp, error) {
	// 验证必需参数
	if in.UserId == 0 {
		l.Error("用户ID为空或无效")
		return nil, errorx.Msg("用户ID无效")
	}

	// 标准化搜索名称
	searchName := ""
	if in.Name != "" {
		searchName = strings.ToLower(strings.TrimSpace(in.Name))
		l.Infof("开始查询用户 [ID:%d] 名称包含 [%s] 的项目列表", in.UserId, searchName)
	} else {
		l.Infof("开始查询用户 [ID:%d] 的所有项目列表", in.UserId)
	}

	// 检查是否为超级管理员
	if l.isSuperAdmin(in.Roles) {
		l.Infof("用户 [ID:%d] 拥有 SUPER_ADMIN 角色，返回所有项目", in.UserId)
		return l.getAllProjects(searchName)
	}

	l.Info("用户角色", in.Roles)
	// 查询用户作为管理员的项目关联记录
	projectAdmins, err := l.queryUserProjectAdmins(in.UserId)
	if err != nil {
		return nil, err
	}

	if len(projectAdmins) == 0 {
		l.Infof("用户 [ID:%d] 未关联任何项目", in.UserId)
		return &pb.GetOnecProjectsByUserIdResp{Data: []*pb.ProjectInfo{}}, nil
	}

	// 查询并过滤项目信息
	projects, err := l.buildProjectList(projectAdmins, searchName)
	if err != nil {
		return nil, err
	}

	l.Infof("成功查询到用户 [ID:%d] 的 %d 个匹配项目", in.UserId, len(projects))

	return &pb.GetOnecProjectsByUserIdResp{Data: projects}, nil
}

// isSuperAdmin 检查角色列表中是否包含 SUPER_ADMIN
func (l *ProjectGetByUserIdLogic) isSuperAdmin(roles []string) bool {
	for _, role := range roles {
		if strings.ToUpper(role) == "SUPER_ADMIN" {
			return true
		}
	}
	return false
}

// getAllProjects 获取所有项目（超级管理员使用）
func (l *ProjectGetByUserIdLogic) getAllProjects(searchName string) (*pb.GetOnecProjectsByUserIdResp, error) {
	// 构建查询条件
	queryStr := ""
	var args []any

	if searchName != "" {
		queryStr = "LOWER(name) LIKE ?"
		args = append(args, "%"+searchName+"%")
	}

	// 查询所有项目
	allProjects, err := l.svcCtx.OnecProjectModel.SearchNoPage(
		l.ctx,
		"created_at",
		false,
		queryStr,
		args...,
	)
	if err != nil {
		l.Errorf("查询所有项目失败: %v", err)
		return nil, errorx.Msg("查询项目列表失败")
	}

	// 构建返回结果
	var projects []*pb.ProjectInfo
	for _, project := range allProjects {
		pbProject, err := l.buildProjectInfo(project)
		if err != nil {
			l.Errorf("构建项目 [ID:%d] 信息失败: %v", project.Id, err)
			continue
		}
		projects = append(projects, pbProject)
	}

	l.Infof("超级管理员查询成功，共 %d 个匹配项目", len(projects))
	return &pb.GetOnecProjectsByUserIdResp{Data: projects}, nil
}

// queryUserProjectAdmins 查询用户的项目管理员关联记录
func (l *ProjectGetByUserIdLogic) queryUserProjectAdmins(userId uint64) ([]*model.OnecProjectAdmin, error) {
	projectAdmins, err := l.svcCtx.OnecProjectAdminModel.SearchNoPage(
		l.ctx,
		"created_at",
		false,
		"user_id = ?",
		userId,
	)
	if err != nil {
		l.Errorf("查询用户 [ID:%d] 的项目管理员关联失败: %v", userId, err)
		return nil, errorx.Msg("查询用户项目权限失败")
	}

	l.Infof("用户 [ID:%d] 关联了 %d 个项目", userId, len(projectAdmins))
	return projectAdmins, nil
}

// buildProjectList 构建项目信息列表
func (l *ProjectGetByUserIdLogic) buildProjectList(projectAdmins []*model.OnecProjectAdmin, searchName string) ([]*pb.ProjectInfo, error) {
	var projects []*pb.ProjectInfo

	for _, admin := range projectAdmins {
		project, err := l.svcCtx.OnecProjectModel.FindOne(l.ctx, admin.ProjectId)
		if err != nil {
			l.Errorf("查询项目 [ID:%d] 详情失败: %v", admin.ProjectId, err)
			continue // 跳过查询失败的项目，不影响其他项目的返回
		}

		// 如果指定了搜索名称，进行模糊匹配过滤
		if searchName != "" && !l.matchProjectName(project.Name, searchName) {
			l.Debugf("项目 [%s] 不匹配搜索条件 [%s]，跳过", project.Name, searchName)
			continue
		}

		// 构建项目信息
		pbProject, err := l.buildProjectInfo(project)
		if err != nil {
			l.Errorf("构建项目 [ID:%d] 信息失败: %v", project.Id, err)
			continue
		}

		projects = append(projects, pbProject)
		l.Debugf("成功获取项目 [%s:%s] 信息", project.Name, project.Uuid)
	}

	return projects, nil
}

// matchProjectName 检查项目名称是否匹配搜索条件
func (l *ProjectGetByUserIdLogic) matchProjectName(projectName, searchName string) bool {
	return strings.Contains(strings.ToLower(projectName), searchName)
}

// buildProjectInfo 构建单个项目信息
func (l *ProjectGetByUserIdLogic) buildProjectInfo(project *model.OnecProject) (*pb.ProjectInfo, error) {
	// 构建项目信息响应
	return &pb.ProjectInfo{
		Id:       project.Id,
		Name:     project.Name,
		Uuid:     project.Uuid,
		IsSystem: project.IsSystem,
	}, nil
}
