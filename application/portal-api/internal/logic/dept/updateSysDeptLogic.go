package dept

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/portal-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type UpdateSysDeptLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUpdateSysDeptLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateSysDeptLogic {
	return &UpdateSysDeptLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UpdateSysDeptLogic) UpdateSysDept(req *types.UpdateSysDeptRequest) (resp string, err error) {
	// 获取上下文中的用户信息
	username, ok := l.ctx.Value("username").(string)
	if !ok || username == "" {
		username = "system"
	}

	// 调用 RPC 服务更新部门
	_, err = l.svcCtx.PortalRpc.DeptUpdate(l.ctx, &pb.UpdateSysDeptReq{
		Id:        req.Id,
		Name:      req.Name,
		ParentId:  req.ParentId,
		Remark:    req.Remark,
		Leader:    req.Leader,
		Phone:     req.Phone,
		Email:     req.Email,
		Status:    req.Status,
		Sort:      req.Sort,
		UpdatedBy: username,
	})
	if err != nil {
		l.Errorf("更新部门失败: operator=%s, deptId=%d, error=%v", username, req.Id, err)
		return "", err
	}

	return "更新部门成功", nil
}
