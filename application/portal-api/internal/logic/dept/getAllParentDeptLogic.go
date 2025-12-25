package dept

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/portal-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetAllParentDeptLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetAllParentDeptLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetAllParentDeptLogic {
	return &GetAllParentDeptLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetAllParentDeptLogic) GetAllParentDept() (resp []types.SysDept, err error) {
	// 调用 RPC 服务获取所有父级部门
	res, err := l.svcCtx.PortalRpc.DeptGetAllParent(l.ctx, &pb.GetAllParentDeptReq{})
	if err != nil {
		l.Errorf("获取所有父级部门失败: error=%v", err)
		return nil, err
	}

	// 转换部门列表
	var depts []types.SysDept
	for _, dept := range res.Data {
		depts = append(depts, types.SysDept{
			Id:        dept.Id,
			Name:      dept.Name,
			ParentId:  dept.ParentId,
			Remark:    dept.Remark,
			Leader:    dept.Leader,
			Phone:     dept.Phone,
			Email:     dept.Email,
			Status:    dept.Status,
			Sort:      dept.Sort,
			CreatedBy: dept.CreatedBy,
			UpdatedBy: dept.UpdatedBy,
			CreatedAt: dept.CreatedAt,
			UpdatedAt: dept.UpdatedAt,
		})
	}

	return depts, nil
}
