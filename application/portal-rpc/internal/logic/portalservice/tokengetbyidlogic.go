package portalservicelogic

import (
	"context"
	"errors"

	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type TokenGetByIdLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewTokenGetByIdLogic(ctx context.Context, svcCtx *svc.ServiceContext) *TokenGetByIdLogic {
	return &TokenGetByIdLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// TokenGetById 根据ID获取Token
func (l *TokenGetByIdLogic) TokenGetById(in *pb.GetSysTokenByIdReq) (*pb.GetSysTokenByIdResp, error) {
	// 参数验证
	if in.Id == 0 {
		l.Error("Token ID不能为空")
		return nil, errorx.Msg("Token ID不能为空")
	}

	// 查询Token
	token, err := l.svcCtx.SysToken.FindOne(l.ctx, in.Id)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			l.Errorf("Token不存在，ID: %d", in.Id)
			return nil, errorx.Msg("Token不存在")
		}
		l.Errorf("查询Token失败: %v", err)
		return nil, errorx.Msg("查询Token失败")
	}

	l.Infof("成功获取Token，名称: %s", token.Name)

	// 转换过期时间
	var expireTime int64
	if token.ExpireTime.Valid {
		expireTime = token.ExpireTime.Time.Unix()
	}

	// 构建响应
	return &pb.GetSysTokenByIdResp{
		Data: &pb.SysToken{
			Id:         token.Id,
			OwnerType:  token.OwnerType,
			OwnerId:    token.OwnerId,
			Token:      token.Token,
			Name:       token.Name,
			Type:       token.Type,
			ExpireTime: expireTime,
			Status:     token.Status,
			CreatedBy:  token.CreatedBy,
			UpdatedBy:  token.UpdatedBy,
			CreatedAt:  token.CreatedAt.Unix(),
			UpdatedAt:  token.UpdatedAt.Unix(),
		},
	}, nil
}
