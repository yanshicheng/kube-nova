// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package inspection

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/manager-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type DeleteInspectionTemplateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 删除巡检模板
func NewDeleteInspectionTemplateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteInspectionTemplateLogic {
	return &DeleteInspectionTemplateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DeleteInspectionTemplateLogic) DeleteInspectionTemplate(req *types.InspectionTemplateDelRequest) (resp string, err error) {
	_, err = l.svcCtx.ManagerRpc.InspectionTemplateDel(l.ctx, &pb.InspectionTemplateDelReq{
		Id:        req.Id,
		UpdatedBy: currentUsername(l.ctx),
	})
	if err != nil {
		return "", err
	}

	return "删除巡检模板成功", nil
}
