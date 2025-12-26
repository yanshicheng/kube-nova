package auth

import (
	"context"
	"net/http"
	"strings"

	"github.com/yanshicheng/kube-nova/application/portal-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/metadata"
)

type LoginLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewLoginLogic(ctx context.Context, svcCtx *svc.ServiceContext) *LoginLogic {
	return &LoginLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *LoginLogic) Login(r *http.Request, req *types.LoginRequest) (resp *types.LoginResponse, err error) {
	// 记录登录尝试日志
	l.Infof("用户登录尝试: username=%s", req.Username)

	// ===== 获取客户端信息 =====
	clientIP := getClientIP(r)
	userAgent := r.Header.Get("User-Agent")

	// 打印调试信息（可选）
	l.Infof("客户端信息 - IP: %s, UserAgent: %s", clientIP, userAgent)

	md := metadata.New(map[string]string{
		"x-real-ip":    clientIP,
		"x-user-agent": userAgent, // 使用自定义 key
	})
	ctx := metadata.NewOutgoingContext(l.ctx, md)

	// 调用 RPC 服务进行用户登录验证（使用新的 ctx）
	res, err := l.svcCtx.SysAuthRpc.GetToken(ctx, &pb.GetTokenRequest{
		Username: req.Username,
		Password: req.Password,
	})
	if err != nil {
		l.Errorf("用户登录失败: username=%s, error=%v", req.Username, err)
		return nil, err
	}

	l.Infof("用户登录成功: username=%s, userId=%d", req.Username, res.UserId)

	// 构造返回响应
	return &types.LoginResponse{
		UserId:   res.UserId,
		Username: res.Username,
		NickName: res.NickName,
		Uuid:     res.Uuid,
		Roles:    res.Roles,
		Token: types.TokenResponse{
			AccessToken:      res.Token.AccessToken,
			AccessExpiresIn:  res.Token.AccessExpiresIn,
			RefreshToken:     res.Token.RefreshToken,
			RefreshExpiresIn: res.Token.RefreshExpiresIn,
		},
	}, nil
}

// ===== 工具函数：获取客户端真实 IP =====
func getClientIP(r *http.Request) string {
	// 1. 优先从 X-Real-IP 获取（Nginx 代理常用）
	ip := r.Header.Get("X-Real-IP")
	if ip != "" {
		return ip
	}

	// 2. 从 X-Forwarded-For 获取（可能有多个 IP，取第一个）
	ip = r.Header.Get("X-Forwarded-For")
	if ip != "" {
		// X-Forwarded-For 格式: client, proxy1, proxy2
		ips := strings.Split(ip, ",")
		if len(ips) > 0 {
			return strings.TrimSpace(ips[0])
		}
	}

	// 3. 从 RemoteAddr 获取（直连 IP）
	ip = r.RemoteAddr
	// RemoteAddr 格式可能是 "IP:Port"，需要去掉端口
	if idx := strings.LastIndex(ip, ":"); idx != -1 {
		ip = ip[:idx]
	}

	return ip
}
