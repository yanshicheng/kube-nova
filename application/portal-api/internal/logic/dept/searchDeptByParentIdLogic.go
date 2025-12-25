package dept

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/portal-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type SearchDeptByParentIdLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewSearchDeptByParentIdLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SearchDeptByParentIdLogic {
	return &SearchDeptByParentIdLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SearchDeptByParentIdLogic) SearchDeptByParentId(req *types.SearchDeptByParentIdRequest) (resp []types.SysDept, err error) {

	// 调用 RPC 服务查询下级部门
	res, err := l.svcCtx.PortalRpc.DeptGetNextLevel(l.ctx, &pb.SearchSysDeptByParentIdReq{
		ParentId: req.ParentId,
	})
	if err != nil {
		l.Errorf("查询下级部门失败: parentId=%d, error=%v", req.ParentId, err)
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
