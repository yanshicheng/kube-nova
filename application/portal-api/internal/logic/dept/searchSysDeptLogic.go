package dept

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/portal-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type SearchSysDeptLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewSearchSysDeptLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SearchSysDeptLogic {
	return &SearchSysDeptLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SearchSysDeptLogic) SearchSysDept(req *types.SearchSysDeptRequest) (resp []types.SysDeptTree, err error) {

	// 调用 RPC 服务搜索部门树
	res, err := l.svcCtx.PortalRpc.DeptSearch(l.ctx, &pb.SearchSysDeptReq{})
	if err != nil {
		l.Errorf("搜索部门树失败: error=%v", err)
		return nil, err
	}

	// 转换部门树
	var convertDeptTree func(*pb.SysDeptTree) types.SysDeptTree
	convertDeptTree = func(pbDept *pb.SysDeptTree) types.SysDeptTree {
		dept := types.SysDeptTree{
			Id:        pbDept.Id,
			Name:      pbDept.Name,
			ParentId:  pbDept.ParentId,
			Remark:    pbDept.Remark,
			Leader:    pbDept.Leader,
			Phone:     pbDept.Phone,
			Email:     pbDept.Email,
			Status:    pbDept.Status,
			Sort:      pbDept.Sort,
			CreatedBy: pbDept.CreatedBy,
			UpdatedBy: pbDept.UpdatedBy,
			CreatedAt: pbDept.CreatedAt,
			UpdatedAt: pbDept.UpdatedAt,
		}
		for _, child := range pbDept.Children {
			dept.Children = append(dept.Children, convertDeptTree(child))
		}
		return dept
	}

	var depts []types.SysDeptTree
	for _, dept := range res.Data {
		depts = append(depts, convertDeptTree(dept))
	}

	return depts, nil
}
