package portalservicelogic

import (
	"context"
	"errors"
	"time"

	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type TokenGetByUserIdLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewTokenGetByUserIdLogic(ctx context.Context, svcCtx *svc.ServiceContext) *TokenGetByUserIdLogic {
	return &TokenGetByUserIdLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 获取某个用户token 方法
func (l *TokenGetByUserIdLogic) TokenGetByUserId(in *pb.GetSysTokenByUserIdReq) (*pb.GetSysTokenByUserIdResp, error) {
	// 记录请求日志
	l.Infof("开始获取用户Token，用户ID: %d", in.UserId)

	// 参数验证
	if in.UserId == 0 {
		l.Error("用户ID不能为空")
		return nil, errorx.Msg("用户ID不能为空")
	}

	// 验证用户是否存在
	user, err := l.svcCtx.SysUser.FindOne(l.ctx, in.UserId)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			l.Errorf("用户不存在，用户ID: %d", in.UserId)
			return nil, errorx.Msg("用户不存在")
		}
		l.Errorf("查询用户失败: %v", err)
		return nil, errorx.Msg("查询用户信息失败")
	}

	l.Infof("找到用户: %s", user.Username)

	// 构建查询条件：查找该用户的有效Token
	queryStr := "`owner_type` = ? AND `owner_id` = ? AND `status` = ?"
	args := []interface{}{1, in.UserId, 1} // owner_type=1表示用户，status=1表示启用

	// 查询用户的有效Token（只返回最新的一个）
	tokens, err := l.svcCtx.SysToken.SearchNoPage(
		l.ctx,
		"created_at", // 按创建时间排序
		false,        // 降序，最新的在前
		queryStr,
		args...,
	)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			l.Infof("用户暂无有效Token，用户ID: %d", in.UserId)
			return &pb.GetSysTokenByUserIdResp{
				Token: "",
			}, nil
		}
		l.Errorf("查询用户Token失败: %v", err)
		return nil, errorx.Msg("查询用户Token失败")
	}

	// 过滤出未过期的Token
	var validToken *model.SysToken
	now := time.Now()
	for _, token := range tokens {
		// 检查Token是否过期
		if token.ExpireTime.Valid && token.ExpireTime.Time.Before(now) {
			l.Infof("Token已过期: %s, 过期时间: %s",
				token.Token, token.ExpireTime.Time.Format("2006-01-02 15:04:05"))
			continue
		}
		// 找到第一个有效的Token
		validToken = token
		break
	}

	if validToken == nil {
		l.Infof("用户所有Token均已过期或无效，用户ID: %d", in.UserId)
		return &pb.GetSysTokenByUserIdResp{
			Token: "",
		}, nil
	}

	l.Infof("成功获取用户Token，Token名称: %s", validToken.Name)

	return &pb.GetSysTokenByUserIdResp{
		Token: validToken.Token,
	}, nil
}
