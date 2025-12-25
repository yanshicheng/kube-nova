package alert

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/manager-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/zeromicro/go-zero/core/logx"
)

type DeleteAlertRuleFileLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 删除告警规则文件
func NewDeleteAlertRuleFileLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteAlertRuleFileLogic {
	return &DeleteAlertRuleFileLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DeleteAlertRuleFileLogic) DeleteAlertRuleFile(req *types.DeleteAlertRuleFileRequest) (resp string, err error) {
	// 调用RPC服务删除告警规则文件
	_, err = l.svcCtx.ManagerRpc.AlertRuleFileDel(l.ctx, &pb.DelAlertRuleFileReq{
		Id:      req.Id,
		Cascade: req.Cascade,
	})

	if err != nil {
		l.Errorf("删除告警规则文件失败: %v", err)
		return "", fmt.Errorf("删除告警规则文件失败: %v", err)
	}

	return "告警规则文件删除成功", nil
}
