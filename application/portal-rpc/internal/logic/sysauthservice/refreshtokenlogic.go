package sysauthservicelogic

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/code"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/common"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"
	"github.com/yanshicheng/kube-nova/pkg/jwt"

	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/svc"
	"github.com/zeromicro/go-zero/core/logx"
)

type RefreshTokenLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewRefreshTokenLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RefreshTokenLogic {
	return &RefreshTokenLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 刷新令牌
func (l *RefreshTokenLogic) RefreshToken(in *pb.RefreshTokenRequest) (*pb.RefreshTokenResponse, error) {
	// 拿到刷新令牌
	if in.RefreshToken == "" {
		return nil, code.RefreshTokenEmptyErr
	}
	jwtR, err := jwt.VerifyToken("Bearer "+in.RefreshToken, l.svcCtx.Config.AuthConfig.RefreshSecret)
	if err != nil {
		return nil, err
	}
	// 查询用户信息
	redisKey := fmt.Sprintf("%s%s", common.UuidKeyPrefix, jwtR.UserName.UserName)
	if common.AllowMultiLogin {
		redisKey = fmt.Sprintf("%s%s:%s", common.UuidKeyPrefix, jwtR.UserName.UserName, jwtR.UserName.UserId)
	}
	key, errs := l.svcCtx.Cache.Get(redisKey)
	if errs != nil {
		l.Errorf("Redis 查询该 uuid 的访问令牌, 错误: %v, uuid: %v", err, redisKey)
		return nil, code.UUIDQueryErr
	}
	if key == "" {
		return nil, code.UUIDNotExistErr
	}
	jwtA, err := jwt.VerifyToken("Bearer "+in.RefreshToken, l.svcCtx.Config.AuthConfig.RefreshSecret)
	if err != nil {
		return nil, err
	}
	accessToken, errs := jwt.CreateJWTToken(&jwt.AccountInfo{
		UserId:   jwtA.UserName.UserId,
		UserName: jwtA.UserName.UserName,
		NickName: jwtA.UserName.NickName,
		Uuid:     jwtA.UserName.Uuid,
		Roles:    jwtA.UserName.Roles,
	}, l.svcCtx.Config.AuthConfig.AccessSecret, jwtA.UserName.Uuid, l.svcCtx.Config.AuthConfig.AccessExpire)
	if errs != nil {
		l.Errorf("生成JWT令牌失败, 错误: %v", err)
		return nil, code.GenerateJWTTokenErr
	}
	errs = l.svcCtx.Cache.Setex(redisKey, accessToken.AccessToken, int(l.svcCtx.Config.AuthConfig.AccessExpire))
	if errs != nil {
		l.Errorf("存储访问令牌到 Redis 失败: %v", err)
		return nil, code.RedisStorageErr
	}
	return &pb.RefreshTokenResponse{
		AccessToken:     accessToken.AccessToken,
		AccessExpiresIn: accessToken.ExpiresAt,
	}, nil
}
