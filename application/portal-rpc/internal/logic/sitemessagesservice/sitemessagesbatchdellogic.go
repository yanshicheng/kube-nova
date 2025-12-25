package sitemessagesservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type SiteMessagesBatchDelLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSiteMessagesBatchDelLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SiteMessagesBatchDelLogic {
	return &SiteMessagesBatchDelLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// SiteMessagesBatchDel 批量软删除站内信
func (l *SiteMessagesBatchDelLogic) SiteMessagesBatchDel(in *pb.BatchDelSiteMessagesReq) (*pb.BatchDelSiteMessagesResp, error) {
	// 参数校验
	if len(in.Ids) == 0 {
		return nil, errorx.Msg("消息ID列表不能为空")
	}

	// 使用事务批量删除
	err := l.svcCtx.SiteMessagesModel.TransCtx(l.ctx, func(ctx context.Context, session sqlx.Session) error {
		for _, id := range in.Ids {
			if id == 0 {
				continue
			}

			// 执行删除
			if err := l.svcCtx.SiteMessagesModel.Delete(ctx, id); err != nil {
				l.Errorf("批量删除站内信失败, id=%d, error=%v", id, err)
				return err
			}
		}
		return nil
	})

	if err != nil {
		return nil, errorx.Msg("批量删除站内信失败")
	}

	return &pb.BatchDelSiteMessagesResp{}, nil
}
