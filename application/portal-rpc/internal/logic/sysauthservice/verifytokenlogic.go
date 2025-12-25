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

type VerifyTokenLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewVerifyTokenLogic(ctx context.Context, svcCtx *svc.ServiceContext) *VerifyTokenLogic {
	return &VerifyTokenLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 验证令牌
func (l *VerifyTokenLogic) VerifyToken(in *pb.VerifyTokenRequest) (*pb.VerifyTokenResponse, error) {
	jwtToken, err := jwt.VerifyToken(in.Token, l.svcCtx.Config.AuthConfig.AccessSecret)
	if err != nil {
		return &pb.VerifyTokenResponse{
			IsValid:      false,
			ErrorType:    int64(err.Code()),
			ErrorMessage: err.Message(),
		}, nil
	}
	redisKey := fmt.Sprintf("%s%s", common.UuidKeyPrefix, jwtToken.UserName.UserName)
	if common.AllowMultiLogin {
		redisKey = fmt.Sprintf("%s%s:%s", common.UuidKeyPrefix, jwtToken.UserName.UserName, jwtToken.UserName.Uuid)
	}
	key, errs := l.svcCtx.Cache.Get(redisKey)
	if errs != nil {
		l.Errorf("Redis 查询该 uuid 的访问令牌, 错误: %v, uuid: %v", err, redisKey)
		return nil, code.UUIDQueryErr
	}
	if key == "" {
		l.Infof("Redis 不存在该 uuid 的访问令牌, 错误: %v , key: %v", key, err)
		return &pb.VerifyTokenResponse{
			IsValid:      false,
			ErrorType:    100099,
			ErrorMessage: "token 已经被禁用，请联系管理员处理!",
		}, nil
	}
	// 判断token是否一致, in.Token 是否包含 key 中的内容

	if "Bearer "+key != in.Token {
		return &pb.VerifyTokenResponse{
			IsValid:      false,
			ErrorType:    int64(jwt.ErrTokenUsed.Code()),
			ErrorMessage: jwt.ErrTokenUsed.Message(),
		}, nil
	}

	return &pb.VerifyTokenResponse{
		IsValid:      true,
		ExpireTime:   jwtToken.ExpiresAt,
		Username:     jwtToken.UserName.UserName,
		UserId:       jwtToken.UserName.UserId,
		Roles:        jwtToken.UserName.Roles,
		NickName:     jwtToken.UserName.NickName,
		Uuid:         jwtToken.UserName.Uuid,
		ErrorType:    0,
		ErrorMessage: "",
	}, nil
}
