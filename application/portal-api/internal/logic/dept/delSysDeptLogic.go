package dept

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/portal-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type DelSysDeptLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewDelSysDeptLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DelSysDeptLogic {
	return &DelSysDeptLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DelSysDeptLogic) DelSysDept(req *types.DefaultIdRequest) (resp string, err error) {
	// 获取上下文中的用户信息
	username, ok := l.ctx.Value("username").(string)
	if !ok || username == "" {
		username = "system"
	}

	// 调用 RPC 服务删除部门
	_, err = l.svcCtx.PortalRpc.DeptDel(l.ctx, &pb.DelSysDeptReq{
		Id: req.Id,
	})
	if err != nil {
		l.Errorf("删除部门失败: operator=%s, deptId=%d, error=%v", username, req.Id, err)
		return "", err
	}

	l.Infof("删除部门成功: operator=%s, deptId=%d", username, req.Id)
	return "删除部门成功", nil
}
