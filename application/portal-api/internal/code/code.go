package code

import "github.com/yanshicheng/kube-nova/common/handler/errorx"

var (
	EncryptPasswordErr = errorx.New(201001, "密码加密失败!")
	// 类型不合法
	ParameterIllegal = errorx.New(201002, "参数不合法!")
	// 不支持的认证类型
	LoginTypeNotSupport = errorx.New(201003, "不支持的认证类型!")
	UUIDNotExistErr     = errorx.New(201004, "uuid查询不到，退出异常!")

	// token内容获取异常
	TokenContentErr = errorx.New(201005, "token内容获取异常!")
)
