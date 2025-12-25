package common

import "time"

const (
	LoginFailKeyPrefix = "login_fail_" // Redis 键前缀

	MaxLoginFailures           = 3                // 最大登录失败次数
	LoginFailExpire            = 5 * time.Minute  // 登录失败记录过期时间
	DefaultRole                = "user"           // 默认角色
	IsResetPasswordFlag        = 1                // 需要重置密码标志
	IsDisabledFlag             = 1                // 账号禁用标志
	UuidKeyPrefix              = "account:token:" // Redis 键前缀
	AllowMultiLogin            = true
	Avatar              string = "/users/20241223/20241223175322.jpg" // 是否允许多端登录
)
