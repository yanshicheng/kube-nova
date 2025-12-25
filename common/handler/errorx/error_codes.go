package errorx

// Defines a set of common error codes and corresponding messages
var (
	OK                 = add(0, "OK")         // OK
	NoLogin            = add(101, "用户未登录")    // NOT_LOGIN
	RequestErr         = add(400, "请求参数错误")   // INVALID_ARGUMENT
	Unauthorized       = add(401, "未认证或认证失败") // UNAUTHENTICATED
	AccessDenied       = add(403, "权限拒绝")     // PERMISSION_DENIED
	NotFound           = add(404, "资源未找到")    // NOT_FOUND
	MethodNotAllowed   = add(405, "方法不被允许")   // METHOD_NOT_ALLOWED
	Canceled           = add(498, "请求已取消")    // CANCELED
	ServerErr          = add(500, "服务器内部错误")  // INTERNAL_ERROR
	ServiceUnavailable = add(503, "服务不可用")    // UNAVAILABLE
	ServerDeadline     = add(504, "请求超时")     // DEADLINE_EXCEEDED
	ServerLimitExceed  = add(509, "超出资源限制")   // RESOURCE_EXHAUSTED
	// 数据库相关错误吗
	DatabaseCreateErr      = add(551, "数据创建失败！")
	DatabaseFindErr        = add(552, "数据查询失败！")
	DatabaseQueryErr       = add(550, "数据查询报错！")
	DatabaseNotFound       = add(551, "数据未找到！")
	DatabaseUpdateErr      = add(553, "数据更新失败！")
	DatabaseDeleteErr      = add(554, "数据删除失败！")
	DatabaseCopyErr        = add(555, "数据复制失败！")
	DatabaseTransactionErr = add(556, "数据库执行事务失败！")
	DatabaseProcessErr     = add(557, "数据处理失败！")

	// 通用错误类型
	// UUID 生成错误
	UUIDGenerateErr = add(601, "UUID 生成错误!")
)
