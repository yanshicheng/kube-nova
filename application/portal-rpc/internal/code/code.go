package code

import "github.com/yanshicheng/kube-nova/common/handler/errorx"

var (
	// 通用错误类型（第四位为 0）
	AccountRequiredParams       = errorx.New(101001, "用户名，账号，图标，手机号，邮箱，工号，入职时间为必传参数!")
	ParameterIllegal            = errorx.New(101002, "参数不合法!")
	GenerateUUIDErr             = errorx.New(101003, "生成 UUID 失败!")
	PasswordIllegal             = errorx.New(101004, "密码必须大于 6 位，包含数字、字母、特殊字符!")
	ActionIllegal               = errorx.New(101005, "action 只能是 GET POST PUT DELETE *!")
	LevelIllegal                = errorx.New(101006, "level 只能是 1 2!")
	ParentPermissionNotExist    = errorx.New(101007, "父级权限不存在!")
	DeletePermissionHasChildErr = errorx.New(101008, "删除权限失败，请先清空子权限!")
	RecordNotFound              = errorx.New(101009, "记录不存在!")
	DatabaseError               = errorx.New(101010, "数据库操作失败!")
	// 数据未查询到
	RecordNotExist    = errorx.New(101012, "记录未查询到!")
	InvalidPageParams = errorx.New(101011, "分页参数不合法!")

	// 账号相关错误（第四位为 1）
	EncryptPasswordErr      = errorx.New(101101, "密码加密失败!")
	CreateAccountErr        = errorx.New(101102, "创建账号失败!")
	FindAccountErr          = errorx.New(101103, "查询账号失败!")
	DecodeBase64PasswordErr = errorx.New(101104, "解码密码失败!")
	LoginErr                = errorx.New(101105, "用户名或密码校验失败!")
	PasswordNotMatchErr     = errorx.New(101106, "密码不匹配或新旧密码一致!")
	ChangePasswordErr       = errorx.New(101107, "修改密码失败!")
	FrozenAccountsErr       = errorx.New(101108, "冻结账号操作失败!")
	AccountLockedErr        = errorx.New(101109, "账号被冻结，请联系管理员!")
	AccountLockedTip        = errorx.New(101110, "账号已经设置为离职状态!")
	ResetPasswordErr        = errorx.New(101111, "重置密码失败!")
	ResetPasswordTip        = errorx.New(101112, "登录失败请重置密码!")
	LeaveErr                = errorx.New(101113, "离职操作失败!")
	UpdateSysUserErr        = errorx.New(101114, "更新用户信息失败!")
	FindSysUserListErr      = errorx.New(101115, "查询用户列表失败!")
	NewPasswordNotMatchErr  = errorx.New(101117, "新密码不匹配!")
	UserNotExistErr         = errorx.New(101118, "用户不存在!")
	UsernameAlreadyExistErr = errorx.New(101119, "用户名已存在!")
	UserStatusDisabledErr   = errorx.New(101120, "用户状态已被禁用!")

	// 角色相关错误（第四位为 2）
	CreateRoleErr             = errorx.New(101201, "创建角色失败!")
	FindRoleErr               = errorx.New(101202, "查询角色失败!")
	BindRoleErr               = errorx.New(101203, "绑定角色失败!")
	FindRolePermissionErr     = errorx.New(101204, "查询角色权限失败!")
	FindRolePermissionListErr = errorx.New(101205, "批量查询角色权限关系失败!")
	DelBindRolePermissionErr  = errorx.New(101206, "删除角色权限关系失败!")
	BindRolePermissionErr     = errorx.New(101207, "绑定角色权限失败!")
	RoleNotExistErr           = errorx.New(101208, "角色不存在!")
	RoleCodeAlreadyExistErr   = errorx.New(101209, "角色编码已存在!")
	DeleteRoleErr             = errorx.New(101210, "删除角色失败!")
	UpdateRoleErr             = errorx.New(101211, "更新角色失败!")
	RoleMenuBindErr           = errorx.New(101212, "角色菜单绑定失败!")
	RoleApiBindErr            = errorx.New(101213, "角色API权限绑定失败!")

	// 权限/API相关错误（第四位为 3）
	GetChildPermissionErr = errorx.New(101301, "获取子集权限异常!")
	FindDictItemsErr      = errorx.New(101302, "查询字典数据失败!")
	DictHasItemsErr       = errorx.New(101303, "字典无法删除，请先删除字典数据!")
	ApiNotExistErr        = errorx.New(101304, "API不存在!")
	CreateApiErr          = errorx.New(101305, "创建API失败!")
	UpdateApiErr          = errorx.New(101306, "更新API失败!")
	DeleteApiErr          = errorx.New(101307, "删除API失败!")
	ApiPathMethodExistErr = errorx.New(101308, "API路径和方法组合已存在!")
	SearchApiErr          = errorx.New(101309, "查询API列表失败!")
	GetApiTreeErr         = errorx.New(101310, "获取API分组树失败!")

	// 部门相关错误（第四位为 4）
	CreateOrganizationErr        = errorx.New(101401, "创建机构层级失败!")
	DeleteOrganizationNotNullErr = errorx.New(101402, "删除机构失败，请先清空子机构!")
	GetOrganizationInfoErr       = errorx.New(101403, "机构信息获取失败!")
	GetOrganizationErr           = errorx.New(101404, "机构信息查询出错!")
	GetPositionInfoErr           = errorx.New(101405, "职位信息获取失败!")
	GetPositionErr               = errorx.New(101406, "职位信息查询出错!")
	DeptNotExistErr              = errorx.New(101407, "部门不存在!")
	CreateDeptErr                = errorx.New(101408, "创建部门失败!")
	UpdateDeptErr                = errorx.New(101409, "更新部门失败!")
	DeleteDeptErr                = errorx.New(101410, "删除部门失败!")
	DeleteDeptHasChildErr        = errorx.New(101411, "删除部门失败，请先删除子部门!")
	SearchDeptErr                = errorx.New(101412, "查询部门列表失败!")
	GetDeptTreeErr               = errorx.New(101413, "获取部门树失败!")
	ParentDeptNotExistErr        = errorx.New(101414, "父级部门不存在!")
	// 查询部门失败
	GetDeptErr = errorx.New(101415, "查询部门失败!")
	// 部门层级检查失败
	CheckDeptHierarchyErr = errorx.New(101416, "部门层级检查失败!")
	// 部门层级超出限制
	DeptHierarchyLimitErr = errorx.New(101417, "部门层级超出限制!")

	// Minio 相关错误（第四位为 5）
	MinioCheckErr  = errorx.New(101501, "Minio 检查失败!")
	MinioUploadErr = errorx.New(101502, "Minio 文件上传失败，请检查 Minio 服务!")

	// Token 相关错误（第四位为 6）
	GenerateJWTTokenErr    = errorx.New(101601, "生成 JWT Token 失败!")
	RedisStorageErr        = errorx.New(101602, "Redis 存储失败!")
	UUIDExistErr           = errorx.New(101603, "UUID 已经存在，Token 获取异常!")
	UUIDNotExistErr        = errorx.New(101604, "UUID 查询不到，退出异常!")
	UUIDDeleteErr          = errorx.New(101605, "UUID 删除失败，退出异常!")
	UUIDQueryErr           = errorx.New(101606, "UUID 查询报错，请联系管理员处理!")
	RefreshTokenEmptyErr   = errorx.New(101607, "刷新令牌不能为空!")
	RefreshTokenExpiredErr = errorx.New(101608, "刷新令牌过期!")
	TokenVerifyErr         = errorx.New(101609, "Token验证失败!")
	TokenExpiredErr        = errorx.New(101610, "Token已过期!")
	GenerateTokenErr       = errorx.New(101611, "生成Token失败!")
	SaveTokenCacheErr      = errorx.New(101612, "保存Token缓存失败!")
	DeleteTokenCacheErr    = errorx.New(101613, "删除Token缓存失败!")

	// 数据字典相关（第四位为 7）
	DictNotExistErr = errorx.New(101701, "字典不存在!")

	// 登录日志相关错误（第四位为 8）
	CreateLoginLogErr = errorx.New(101801, "创建登录日志失败!")
	DeleteLoginLogErr = errorx.New(101802, "删除登录日志失败!")
	SearchLoginLogErr = errorx.New(101803, "查询登录日志失败!")

	// 菜单相关错误（第四位为 9）
	MenuNotExistErr       = errorx.New(101901, "菜单不存在!")
	CreateMenuErr         = errorx.New(101902, "创建菜单失败!")
	UpdateMenuErr         = errorx.New(101903, "更新菜单失败!")
	DeleteMenuErr         = errorx.New(101904, "删除菜单失败!")
	DeleteMenuHasChildErr = errorx.New(101905, "删除菜单失败，请先删除子菜单!")
	SearchMenuErr         = errorx.New(101906, "查询菜单列表失败!")
	GetMenuTreeErr        = errorx.New(101907, "获取菜单树失败!")
	GetMenuChildrenErr    = errorx.New(101908, "获取子菜单失败!")
	ParentMenuNotExistErr = errorx.New(101909, "父级菜单不存在!")

	// 文件上传相关错误（第五位为 A）
	FileUploadErr      = errorx.New(101910, "文件上传失败!")
	FileTypeNotAllowed = errorx.New(101911, "文件类型不允许!")
	FileSizeExceeded   = errorx.New(101912, "文件大小超出限制!")
	FileStorageErr     = errorx.New(101913, "文件存储失败!")
	FileNotFoundErr    = errorx.New(101914, "文件不存在!")
)
