package dept

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/portal-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetSysDeptByIdLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetSysDeptByIdLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetSysDeptByIdLogic {
	return &GetSysDeptByIdLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetSysDeptByIdLogic) GetSysDeptById(req *types.DefaultIdRequest) (resp *types.SysDept, err error) {

	// 调用 RPC 服务获取部门信息
	res, err := l.svcCtx.PortalRpc.DeptGetById(l.ctx, &pb.GetSysDeptByIdReq{
		Id: req.Id,
	})
	if err != nil {
		l.Errorf("获取部门失败: deptId=%d, error=%v", req.Id, err)
		return nil, err
	}

	return &types.SysDept{
		Id:        res.Data.Id,
		Name:      res.Data.Name,
		ParentId:  res.Data.ParentId,
		Remark:    res.Data.Remark,
		Leader:    res.Data.Leader,
		Phone:     res.Data.Phone,
		Email:     res.Data.Email,
		Status:    res.Data.Status,
		Sort:      res.Data.Sort,
		CreatedBy: res.Data.CreatedBy,
		UpdatedBy: res.Data.UpdatedBy,
		CreatedAt: res.Data.CreatedAt,
		UpdatedAt: res.Data.UpdatedAt,
	}, nil
}
