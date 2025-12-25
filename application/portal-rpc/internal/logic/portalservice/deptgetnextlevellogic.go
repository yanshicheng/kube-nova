package portalservicelogic

import (
	"context"
	"errors"

	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type DeptGetNextLevelLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewDeptGetNextLevelLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeptGetNextLevelLogic {
	return &DeptGetNextLevelLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 根据 父id 查询 下一级 部门
func (l *DeptGetNextLevelLogic) DeptGetNextLevel(in *pb.SearchSysDeptByParentIdReq) (*pb.SearchSysDeptByParentIdResp, error) {
	// 如果传入的父ID大于0，需要验证父部门是否存在
	if in.ParentId > 0 {
		_, err := l.svcCtx.SysDept.FindOne(l.ctx, in.ParentId)
		if err != nil {
			if errors.Is(err, model.ErrNotFound) {
				l.Errorf("查询下一级部门失败：父部门不存在, parentId: %d", in.ParentId)
				return nil, errorx.Msg("父部门不存在")
			}
			l.Errorf("查询父部门信息失败: %v", err)
			return nil, errorx.Msg("查询父部门信息失败")
		}
	}

	// 查询下一级部门（按sort字段升序排列）
	subDepts, err := l.svcCtx.SysDept.SearchNoPage(l.ctx, "sort", true, "`parent_id` = ?", in.ParentId)
	if err != nil && !errors.Is(err, model.ErrNotFound) {
		l.Errorf("查询下一级部门失败: %v", err)
		return nil, errorx.Msg("查询下一级部门失败")
	}

	// 转换为响应格式
	var pbDepts []*pb.SysDept
	for _, dept := range subDepts {
		pbDept := l.convertToPbDept(dept)
		pbDepts = append(pbDepts, pbDept)
	}

	return &pb.SearchSysDeptByParentIdResp{
		Data: pbDepts,
	}, nil
}

// convertToPbDept 将数据库模型转换为protobuf格式
func (l *DeptGetNextLevelLogic) convertToPbDept(dept *model.SysDept) *pb.SysDept {
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
