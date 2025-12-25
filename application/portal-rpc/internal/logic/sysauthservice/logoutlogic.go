package sysauthservicelogic

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/code"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/common"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type LogoutLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewLogoutLogic(ctx context.Context, svcCtx *svc.ServiceContext) *LogoutLogic {
	return &LogoutLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 注销
func (l *LogoutLogic) Logout(in *pb.LogoutRequest) (*pb.LogoutResponse, error) {
	// 先判断 redis 是否已经有这个 uuid 访问令牌了
	redisKey := fmt.Sprintf("%s%s", common.UuidKeyPrefix, in.Username)
	if common.AllowMultiLogin {
		redisKey = fmt.Sprintf("%s%s:%s", common.UuidKeyPrefix, in.Username, in.Uuid)
	}
	key, err := l.svcCtx.Cache.Get(redisKey)
	if err != nil {
		l.Errorf("Redis 查询该 uuid 的访问令牌, 错误: %v, uuid: %v", err, redisKey)
		return nil, code.UUIDQueryErr
	}
	if key == "" {
		l.Infof("Redis 不存在该 uuid 的访问令牌, 错误: %v , key: %v", key, err)
		return nil, code.UUIDNotExistErr
	}
	// 删除 redis 中该 uuid 的访问令牌
	if _, err := l.svcCtx.Cache.Del(redisKey); err != nil {
		l.Errorf("删除 Redis 中 uuid 的访问令牌失败, 错误: %v", err)
		return nil, code.UUIDDeleteErr
	}
	return &pb.LogoutResponse{}, nil
}
