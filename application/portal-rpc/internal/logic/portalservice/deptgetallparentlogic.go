package portalservicelogic

import (
	"context"
	"errors"

	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/svc"
	"github.com/zeromicro/go-zero/core/logx"
)

type DeptGetAllParentLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewDeptGetAllParentLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeptGetAllParentLogic {
	return &DeptGetAllParentLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 查询所有父级 部门
func (l *DeptGetAllParentLogic) DeptGetAllParent(in *pb.GetAllParentDeptReq) (*pb.GetAllParentDeptResp, error) {
	// 查询所有父级部门（parent_id = 0），按sort字段升序排列
	parentDepts, err := l.svcCtx.SysDept.SearchNoPage(l.ctx, "sort", true, "`parent_id` = ?", 0)
	if err != nil && !errors.Is(err, model.ErrNotFound) {
		l.Errorf("查询所有父级部门失败: %v", err)
		return nil, errorx.Msg("查询父级部门失败")
	}

	// 转换为响应格式
	var pbDepts []*pb.SysDept
	for _, dept := range parentDepts {
		pbDept := l.convertToPbDept(dept)
		pbDepts = append(pbDepts, pbDept)
	}

	return &pb.GetAllParentDeptResp{
		Data: pbDepts,
	}, nil
}

// convertToPbDept 将数据库模型转换为protobuf格式
func (l *DeptGetAllParentLogic) convertToPbDept(dept *model.SysDept) *pb.SysDept {
	return &pb.SysDept{
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
		CreatedAt: dept.CreatedAt.Unix(),
		UpdatedAt: dept.UpdatedAt.Unix(),
	}
}
