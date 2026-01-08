package operator

import (
	"context"
	"fmt"
	"net/url"
	"strconv"

	"github.com/yanshicheng/kube-nova/application/console-rpc/internal/repositorymanager/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type MemberOperatorImpl struct {
	log logx.Logger
	ctx context.Context
	*BaseOperator
}

func NewMemberOperator(ctx context.Context, base *BaseOperator) types.MemberOperator {
	return &MemberOperatorImpl{
		log:          logx.WithContext(ctx),
		ctx:          ctx,
		BaseOperator: base,
	}
}

func (m *MemberOperatorImpl) List(projectNameOrID string, req types.ListRequest) (*types.ListProjectMemberResponse, error) {
	m.log.Infof("列出项目成员: project=%s, search=%s, page=%d, pageSize=%d",
		projectNameOrID, req.Search, req.Page, req.PageSize)

	// 设置默认值
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 10
	}
	if req.PageSize > 100 {
		req.PageSize = 100
	}

	// 构建查询参数
	params := map[string]string{
		"page":      strconv.FormatInt(req.Page, 10),
		"page_size": strconv.FormatInt(req.PageSize, 10),
	}

	// 搜索参数 - Harbor 支持 entityname 参数
	if req.Search != "" {
		params["entityname"] = req.Search
	}

	query := m.buildQuery(params)
	path := fmt.Sprintf("/api/v2.0/projects/%s/members%s", url.PathEscape(projectNameOrID), query)

	m.log.Debugf("请求路径: %s", path)

	var members []types.ProjectMember
	if err := m.doRequest("GET", path, nil, &members); err != nil {
		return nil, err
	}

	m.log.Infof("API 返回 %d 个项目成员", len(members))

	// 估算总数
	total := len(members)
	if len(members) == int(req.PageSize) {
		total = int(req.Page * req.PageSize)
	}

	totalPages := (total + int(req.PageSize) - 1) / int(req.PageSize)

	m.log.Infof("返回成员列表: total=%d, page=%d/%d, 返回=%d",
		total, req.Page, totalPages, len(members))

	return &types.ListProjectMemberResponse{
		ListResponse: types.ListResponse{
			Total:      total,
			Page:       req.Page,
			PageSize:   req.PageSize,
			TotalPages: totalPages,
		},
		Items: members,
	}, nil
}

// Get 获取项目成员 - 保持不变
func (m *MemberOperatorImpl) Get(projectNameOrID string, memberID int64) (*types.ProjectMember, error) {
	m.log.Infof("获取项目成员: project=%s, memberID=%d", projectNameOrID, memberID)

	var member types.ProjectMember
	path := fmt.Sprintf("/api/v2.0/projects/%s/members/%d",
		url.PathEscape(projectNameOrID), memberID)

	if err := m.doRequest("GET", path, nil, &member); err != nil {
		return nil, err
	}

	m.log.Infof("获取成员成功: %s (role: %s)", member.EntityName, member.RoleName)
	return &member, nil
}

// Add 添加项目成员 - 保持不变
func (m *MemberOperatorImpl) Add(projectNameOrID string, req *types.ProjectMemberReq) (int64, error) {
	m.log.Infof("添加项目成员: project=%s, user=%s, roleID=%d",
		projectNameOrID, req.MemberUser, req.RoleID)

	if !ValidateRoleID(req.RoleID) {
		m.log.Errorf("无效的角色ID: %d", req.RoleID)
		return 0, fmt.Errorf("无效的角色ID: %d (有效范围: 1-4)", req.RoleID)
	}

	body := map[string]interface{}{
		"member_user": map[string]string{
			"username": req.MemberUser,
		},
		"role_id": req.RoleID,
	}

	path := fmt.Sprintf("/api/v2.0/projects/%s/members", url.PathEscape(projectNameOrID))

	if err := m.doRequest("POST", path, body, nil); err != nil {
		return 0, err
	}

	m.log.Infof("添加成员成功: %s (role: %s)", req.MemberUser, GetRoleName(req.RoleID))
	return 0, nil
}

// Update 更新项目成员 - 保持不变
func (m *MemberOperatorImpl) Update(projectNameOrID string, memberID int64, req *types.ProjectMemberReq) error {
	m.log.Infof("更新项目成员: project=%s, memberID=%d, roleID=%d",
		projectNameOrID, memberID, req.RoleID)

	if !ValidateRoleID(req.RoleID) {
		m.log.Errorf("无效的角色ID: %d", req.RoleID)
		return fmt.Errorf("无效的角色ID: %d (有效范围: 1-4)", req.RoleID)
	}

	body := map[string]interface{}{
		"role_id": req.RoleID,
	}

	path := fmt.Sprintf("/api/v2.0/projects/%s/members/%d",
		url.PathEscape(projectNameOrID), memberID)

	if err := m.doRequest("PUT", path, body, nil); err != nil {
		return err
	}

	m.log.Infof("更新成员角色成功: memberID=%d (new role: %s)", memberID, GetRoleName(req.RoleID))
	return nil
}

// Delete 删除项目成员 - 保持不变
func (m *MemberOperatorImpl) Delete(projectNameOrID string, memberID int64) error {
	m.log.Infof("删除项目成员: project=%s, memberID=%d", projectNameOrID, memberID)

	path := fmt.Sprintf("/api/v2.0/projects/%s/members/%d",
		url.PathEscape(projectNameOrID), memberID)

	if err := m.doRequest("DELETE", path, nil, nil); err != nil {
		return err
	}

	m.log.Infof("删除成员成功: memberID=%d", memberID)
	return nil
}
