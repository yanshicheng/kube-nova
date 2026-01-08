package operator

import (
	"context"
	"fmt"
	"strconv"

	"github.com/yanshicheng/kube-nova/application/console-rpc/internal/repositorymanager/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type UserOperatorImpl struct {
	log logx.Logger
	ctx context.Context
	*BaseOperator
}

func NewUserOperator(ctx context.Context, base *BaseOperator) types.UserOperator {
	return &UserOperatorImpl{
		log:          logx.WithContext(ctx),
		ctx:          ctx,
		BaseOperator: base,
	}
}

func (u *UserOperatorImpl) List(req types.ListRequest) (*types.ListUserResponse, error) {
	u.log.Infof("列出用户: search=%s, page=%d, pageSize=%d", req.Search, req.Page, req.PageSize)

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

	// 搜索参数 - Harbor 支持 username 参数进行精确搜索
	if req.Search != "" {
		params["username"] = req.Search
	}

	query := u.buildQuery(params)
	path := "/api/v2.0/users" + query

	u.log.Debugf("请求路径: %s", path)

	var users []types.HarborUser
	if err := u.doRequest("GET", path, nil, &users); err != nil {
		return nil, err
	}

	u.log.Infof("API 返回 %d 个用户", len(users))

	// 估算总数
	total := len(users)
	if len(users) == int(req.PageSize) {
		total = int(req.Page * req.PageSize)
	}

	totalPages := (total + int(req.PageSize) - 1) / int(req.PageSize)

	u.log.Infof("返回用户列表: total=%d, page=%d/%d, 返回=%d",
		total, req.Page, totalPages, len(users))

	return &types.ListUserResponse{
		ListResponse: types.ListResponse{
			Total:      total,
			Page:       req.Page,
			PageSize:   req.PageSize,
			TotalPages: totalPages,
		},
		Items: users,
	}, nil
}

// Get 通过用户 ID 获取用户 - 保持不变
func (u *UserOperatorImpl) Get(userID int64) (*types.HarborUser, error) {
	u.log.Infof("获取用户: userID=%d", userID)

	var user types.HarborUser
	path := fmt.Sprintf("/api/v2.0/users/%d", userID)

	if err := u.doRequest("GET", path, nil, &user); err != nil {
		return nil, err
	}

	u.log.Infof("获取用户成功: %s", user.Username)
	return &user, nil
}

func (u *UserOperatorImpl) GetByUsername(username string) (*types.HarborUser, error) {
	u.log.Infof("通过用户名获取用户: username=%s", username)

	params := map[string]string{
		"username":  username,
		"page":      "1",
		"page_size": "1",
	}
	query := u.buildQuery(params)
	path := "/api/v2.0/users" + query

	var users []types.HarborUser
	if err := u.doRequest("GET", path, nil, &users); err != nil {
		return nil, err
	}

	if len(users) == 0 {
		return nil, fmt.Errorf("用户不存在: %s", username)
	}

	u.log.Infof("获取用户成功: %s (ID: %d)", users[0].Username, users[0].UserID)
	return &users[0], nil
}

// Create 创建用户 - 保持不变
func (u *UserOperatorImpl) Create(req *types.UserReq) (int64, error) {
	u.log.Infof("创建用户: username=%s, email=%s", req.Username, req.Email)

	if req.Username == "" || req.Email == "" || req.Realname == "" || req.Password == "" {
		return 0, fmt.Errorf("用户名、邮箱、全名和密码不能为空")
	}

	body := map[string]interface{}{
		"username": req.Username,
		"email":    req.Email,
		"realname": req.Realname,
		"password": req.Password,
	}

	if req.Comment != "" {
		body["comment"] = req.Comment
	}

	path := "/api/v2.0/users"

	if err := u.doRequest("POST", path, body, nil); err != nil {
		return 0, err
	}

	u.log.Infof("创建用户成功: %s", req.Username)

	user, err := u.GetByUsername(req.Username)
	if err != nil {
		u.log.Errorf("获取新建用户失败: %v", err)
		return 0, nil
	}

	return user.UserID, nil
}

// Update 更新用户 - 保持不变
func (u *UserOperatorImpl) Update(userID int64, req *types.UserUpdateReq) error {
	u.log.Infof("更新用户: userID=%d", userID)

	body := make(map[string]interface{})

	if req.Email != "" {
		body["email"] = req.Email
	}
	if req.Realname != "" {
		body["realname"] = req.Realname
	}
	if req.Comment != "" {
		body["comment"] = req.Comment
	}

	if len(body) == 0 {
		return fmt.Errorf("没有需要更新的字段")
	}

	path := fmt.Sprintf("/api/v2.0/users/%d", userID)

	if err := u.doRequest("PUT", path, body, nil); err != nil {
		return err
	}

	u.log.Infof("更新用户成功: userID=%d", userID)
	return nil
}

// Delete 删除用户 - 保持不变
func (u *UserOperatorImpl) Delete(userID int64) error {
	u.log.Infof("删除用户: userID=%d", userID)

	path := fmt.Sprintf("/api/v2.0/users/%d", userID)

	if err := u.doRequest("DELETE", path, nil, nil); err != nil {
		return err
	}

	u.log.Infof("删除用户成功: userID=%d", userID)
	return nil
}

// ChangePassword 修改用户密码 - 保持不变
func (u *UserOperatorImpl) ChangePassword(userID int64, req *types.PasswordReq) error {
	u.log.Infof("修改用户密码: userID=%d", userID)

	if req.OldPassword == "" || req.NewPassword == "" {
		return fmt.Errorf("旧密码和新密码不能为空")
	}

	body := map[string]interface{}{
		"old_password": req.OldPassword,
		"new_password": req.NewPassword,
	}

	path := fmt.Sprintf("/api/v2.0/users/%d/password", userID)

	if err := u.doRequest("PUT", path, body, nil); err != nil {
		return err
	}

	u.log.Infof("修改密码成功: userID=%d", userID)
	return nil
}

// SetAdmin 设置用户管理员权限 - 保持不变
func (u *UserOperatorImpl) SetAdmin(userID int64, isAdmin bool) error {
	u.log.Infof("设置用户管理员权限: userID=%d, isAdmin=%v", userID, isAdmin)

	body := map[string]interface{}{
		"sysadmin_flag": isAdmin,
	}

	path := fmt.Sprintf("/api/v2.0/users/%d/sysadmin", userID)

	if err := u.doRequest("PUT", path, body, nil); err != nil {
		return err
	}

	action := "取消"
	if isAdmin {
		action = "设置为"
	}
	u.log.Infof("%s管理员成功: userID=%d", action, userID)
	return nil
}
