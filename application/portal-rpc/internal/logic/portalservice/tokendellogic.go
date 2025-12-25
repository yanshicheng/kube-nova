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

type TokenDelLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewTokenDelLogic(ctx context.Context, svcCtx *svc.ServiceContext) *TokenDelLogic {
	return &TokenDelLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *TokenDelLogic) TokenDel(in *pb.DelSysTokenReq) (*pb.DelSysTokenResp, error) {

	// 参数验证
	if in.Id == 0 {
		l.Error("Token ID不能为空")
		return nil, errorx.Msg("Token ID不能为空")
	}

	// 检查Token是否存在
	token, err := l.svcCtx.SysToken.FindOne(l.ctx, in.Id)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			l.Errorf("Token不存在，ID: %d", in.Id)
			return nil, errorx.Msg("Token不存在")
		}
		l.Errorf("查询Token失败: %v", err)
		return nil, errorx.Msg("查询Token失败")
	}

	l.Infof("找到Token，名称: %s, Token值: %s", token.Name, token.Token)

	// 执行软删除
	err = l.svcCtx.SysToken.Delete(l.ctx, in.Id)
	if err != nil {
		l.Errorf("删除Token失败: %v", err)
		return nil, errorx.Msg("删除Token失败")
	}

	l.Infof("成功删除Token，ID: %d", in.Id)

	return &pb.DelSysTokenResp{}, nil
}
