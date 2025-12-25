package dept

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/portal-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type AddSysDeptLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAddSysDeptLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AddSysDeptLogic {
	return &AddSysDeptLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AddSysDeptLogic) AddSysDept(req *types.AddSysDeptRequest) (resp string, err error) {
	// 获取上下文中的用户信息
	username, ok := l.ctx.Value("username").(string)
	if !ok || username == "" {
		username = "system"
	}

	// 调用 RPC 服务添加部门
	_, err = l.svcCtx.PortalRpc.DeptAdd(l.ctx, &pb.AddSysDeptReq{
		Name:      req.Name,
		ParentId:  req.ParentId,
		Remark:    req.Remark,
		Leader:    req.Leader,
		Phone:     req.Phone,
		Email:     req.Email,
		Status:    req.Status,
		Sort:      req.Sort,
		CreatedBy: username,
		UpdatedBy: username,
	})
	if err != nil {
		l.Errorf("添加部门失败: operator=%s, deptName=%s, error=%v", username, req.Name, err)
		return "", err
	}

	l.Infof("添加部门成功: operator=%s, deptName=%s", username, req.Name)
	return "添加部门成功", nil
}
