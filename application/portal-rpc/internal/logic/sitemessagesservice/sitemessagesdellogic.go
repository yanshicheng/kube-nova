package sitemessagesservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type SiteMessagesDelLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSiteMessagesDelLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SiteMessagesDelLogic {
	return &SiteMessagesDelLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// SiteMessagesDel 软删除站内信
func (l *SiteMessagesDelLogic) SiteMessagesDel(in *pb.DelSiteMessagesReq) (*pb.DelSiteMessagesResp, error) {
	// 参数校验
	if in.Id == 0 {
		return nil, errorx.Msg("消息ID不能为空")
	}

	// 执行删除
	err := l.svcCtx.SiteMessagesModel.Delete(l.ctx, in.Id)
	if err != nil {
		l.Errorf("删除站内信失败: %v", err)
		return nil, errorx.Msg("删除站内信失败")
	}

	return &pb.DelSiteMessagesResp{}, nil
}
