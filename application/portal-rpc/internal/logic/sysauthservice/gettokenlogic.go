package sysauthservicelogic

import (
	"context"
	"fmt"
	"time"

	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/code"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/common"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"
	"github.com/yanshicheng/kube-nova/common/vars"
	"github.com/yanshicheng/kube-nova/pkg/jwt"
	"github.com/yanshicheng/kube-nova/pkg/utils"

	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/svc"
	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/metadata"
)

type GetTokenLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetTokenLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetTokenLogic {
	return &GetTokenLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 获取令牌
func (l *GetTokenLogic) GetToken(in *pb.GetTokenRequest) (*pb.GetTokenResponse, error) {
	// 查询账号
	user, err := l.svcCtx.SysUser.FindOneByUsername(l.ctx, in.Username)
	if err != nil {
		l.Errorf("查询账号失败, 账号: %s, 错误: %v", in.Username, err)
		// 记录登录失败日志
		l.recordLoginLog(0, in.Username, 0)
		return nil, code.FindAccountErr
	}
	// 验证是否需要重置密码
	if user.IsNeedResetPwd == 1 {
		l.Infof("账号需要重置密码, 账号: %s", in.Username)
		// 记录登录失败日志
		l.recordLoginLog(user.Id, user.Username, 0)
		return nil, code.ResetPasswordTip
	}
	// 检查账号是否被冻结
	if user.Status == 0 {
		l.Infof("账号已被冻结, 账号: %s", in.Username)
		// 记录登录失败日志
		l.recordLoginLog(user.Id, user.Username, 0)
		return nil, code.FrozenAccountsErr
	}
	// 处理密码验证逻辑
	if err := l.verifyPassword(in.Password, user.Password); err != nil {
		// 如果密码验证失败，记录登录失败次数
		if err := l.recordLoginFailure(user); err != nil {
			l.Errorf("记录登录失败次数时发生错误, 账号: %s, 错误: %v", in.Username, err)
			return nil, code.ChangePasswordErr
		}
		// 记录登录失败日志
		l.recordLoginLog(user.Id, user.Username, 0)
		return nil, code.LoginErr
	}

	// 查询所有角色信息
	sqlStr := "user_id = ?"
	roles, err := l.svcCtx.SysUserRole.SearchNoPage(l.ctx, "id", false, sqlStr, user.Id)
	var rolesNames []string
	var rolesIds []uint64
	if err != nil {
		// 查询过程中的其他错误
		l.Errorf("查询角色信息失败，用户=%d，错误信息：%v", user.Id, err)
		return nil, errorx.DatabaseQueryErr
	}
	if len(roles) == 0 {
		rolesNames = append(rolesNames, "user")
		rolesNames = append(rolesNames, "R_SUPER")
	}
	for _, userRole := range roles {
		role, err := l.svcCtx.SysRole.FindOne(l.ctx, userRole.RoleId)
		if err != nil {
			continue
		}
		rolesNames = append(rolesNames, role.Code)
		rolesIds = append(rolesIds, role.Id)
	}
	l.Infof("账号登录成功, 账号: %s", in.Username)
	uuid, err := utils.GenerateRandomID()
	if err != nil {
		l.Errorf("生成UUID失败, 错误: %v", err)
		return nil, code.GenerateUUIDErr
	}
	accessToken, err := jwt.CreateJWTToken(&jwt.AccountInfo{
		UserId:   user.Id,
		UserName: user.Username,
		NickName: user.Nickname,
		Uuid:     uuid,
		Roles:    rolesNames,
	}, vars.AccessSecret, uuid, vars.AccessExpire)
	if err != nil {
		l.Errorf("生成JWT令牌失败, 错误: %v", err)
		return nil, code.GenerateJWTTokenErr
	}

	// 多端登录模式：允许多端 Token 存储
	redisKey := fmt.Sprintf("%s%s", common.UuidKeyPrefix, user.Username)
	if common.AllowMultiLogin {
		redisKey = fmt.Sprintf("%s%s:%s", common.UuidKeyPrefix, user.Username, uuid)
	}
	err = l.svcCtx.Cache.Setex(redisKey, accessToken.AccessToken, int(vars.AccessExpire))
	if err != nil {
		l.Errorf("存储访问令牌到 Redis 失败: %v", err)
		return nil, code.RedisStorageErr
	}
	refreshToken, err := jwt.CreateJWTToken(&jwt.AccountInfo{
		UserName: user.Username,
		Uuid:     uuid,
	}, vars.RefreshSecret, uuid, vars.RefreshExpire)
	if err != nil {
		l.Errorf("生成JWT令牌失败, 错误: %v", err)
		return nil, code.GenerateJWTTokenErr
	}

	// 记录登录成功日志
	l.recordLoginLog(user.Id, user.Username, 1)

	return &pb.GetTokenResponse{
		Username: user.Username,
		UserId:   user.Id,
		NickName: user.Nickname,
		Roles:    rolesNames,
		Token: &pb.TokenResponse{
			AccessToken:      accessToken.AccessToken,
			RefreshToken:     refreshToken.AccessToken,
			AccessExpiresIn:  accessToken.ExpiresAt,
			RefreshExpiresIn: refreshToken.ExpiresAt,
		},
	}, nil
}

// 验证密码是否正确
func (l *GetTokenLogic) verifyPassword(inputPassword, hashedPassword string) error {
	// 解码密码
	password, err := utils.DecodeBase64Password(inputPassword)
	if err != nil {
		l.Errorf("密码解码失败: %v", err)
		return code.DecodeBase64PasswordErr
	}

	// 校验密码
	if !utils.CheckPasswordHash(password, hashedPassword) {
		l.Infof("密码验证失败")
		return code.LoginErr
	}

	return nil
}

// 记录登录失败次数，如果 5 分钟内连续失败 3 次则禁用账号
func (l *GetTokenLogic) recordLoginFailure(user *model.SysUser) error {
	// Redis 键，存储账号的登录失败次数
	loginFailKey := fmt.Sprintf("%s%s", common.LoginFailKeyPrefix, user.Username)

	// 递增登录失败次数，同时检查当前次数
	failCount, err := l.svcCtx.Cache.Incr(loginFailKey)
	if err != nil {
		return fmt.Errorf("redis Incr 操作失败: %w", err)
	}

	// 如果这是第一次失败，设置失败次数的过期时间为 5 分钟
	if failCount == 1 {
		if err := l.svcCtx.Cache.Expire(loginFailKey, int(common.LoginFailExpire.Seconds())); err != nil {
			return fmt.Errorf("redis 设置过期时间失败: %w", err)
		}
	}

	// 检查是否达到最大失败次数
	if failCount >= common.MaxLoginFailures {
		// 禁用账号的业务逻辑（可以通过数据库标记账号为禁用）
		if err := l.disableUsername(user); err != nil {
			return fmt.Errorf("禁用账号失败: %w", err)
		}
		// 删除 Redis 中的失败记录，避免重复触发
		if _, err := l.svcCtx.Cache.Del(loginFailKey); err != nil {
			return fmt.Errorf("删除 Redis 登录失败记录失败: %w", err)
		}

		l.Infof("账号已被禁用, 账号: %s", user.Username)
		return code.AccountLockedErr
	}

	l.Infof("登录失败计数: %d, 账号: %s", failCount, user.Username)
	return nil
}

// 禁用账号（可以根据具体业务实现，例如更新数据库状态）
func (l *GetTokenLogic) disableUsername(user *model.SysUser) error {
	// 示例实现：将账号设置为禁用状态
	user.Status = 0
	err := l.svcCtx.SysUser.Update(l.ctx, user)
	if err != nil {
		l.Errorf("禁用账号失败, 账号: %s, 错误: %v", user.Username, err)
		return err
	}
	return nil
}

// 记录登录日志
func (l *GetTokenLogic) recordLoginLog(userId uint64, username string, loginStatus int64) {
	// 从 ctx 中获取 IP 和 UserAgent
	ipAddress := l.getClientIPFromCtx()
	userAgent := l.getUserAgentFromCtx()

	// 打印调试信息
	l.Infof("记录登录日志 - 用户: %s, 状态: %d, IP: %s, UserAgent: %s", username, loginStatus, ipAddress, userAgent)

	loginLog := &model.SysLoginLog{
		UserId:      userId,
		Username:    username,
		LoginStatus: loginStatus, // 0-失败, 1-成功
		IpAddress:   ipAddress,
		UserAgent:   userAgent,
		LoginTime:   time.Now(),
		IsDeleted:   0,
	}

	_, err := l.svcCtx.SysLoginLog.Insert(l.ctx, loginLog)
	if err != nil {
		// 记录日志失败不应该影响主流程，只记录错误
		l.Errorf("记录登录日志失败, 用户: %s, 状态: %d, 错误: %v", username, loginStatus, err)
	}
}

// 从 context 中获取客户端 IP
func (l *GetTokenLogic) getClientIPFromCtx() string {
	// 从 gRPC metadata 中获取
	if md, ok := metadata.FromIncomingContext(l.ctx); ok {
		// 优先从 x-real-ip 获取
		if ips := md.Get("x-real-ip"); len(ips) > 0 && ips[0] != "" {
			return ips[0]
		}
		// 从 x-forwarded-for 获取
		if ips := md.Get("x-forwarded-for"); len(ips) > 0 && ips[0] != "" {
			return ips[0]
		}
	}

	// 从 context Value 中获取
	if ip, ok := l.ctx.Value("clientIP").(string); ok && ip != "" {
		return ip
	}

	return "unknown"
}

// 从 context 中获取 UserAgent
func (l *GetTokenLogic) getUserAgentFromCtx() string {
	// 从 gRPC metadata 中获取（使用自定义 key，避免被 gRPC 覆盖）
	if md, ok := metadata.FromIncomingContext(l.ctx); ok {
		if uas := md.Get("x-user-agent"); len(uas) > 0 && uas[0] != "" {
			return uas[0]
		}
	}

	// 从 context Value 中获取
	if ua, ok := l.ctx.Value("userAgent").(string); ok && ua != "" {
		return ua
	}

	return "unknown"
}
