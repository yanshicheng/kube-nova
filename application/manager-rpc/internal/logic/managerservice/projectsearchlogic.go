package managerservicelogic

import (
	"context"
	"errors"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"
	"github.com/yanshicheng/kube-nova/common/vars"

	"github.com/zeromicro/go-zero/core/logx"
)

type ProjectSearchLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewProjectSearchLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ProjectSearchLogic {
	return &ProjectSearchLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ProjectSearch 搜索项目列表
func (l *ProjectSearchLogic) ProjectSearch(in *pb.SearchOnecProjectReq) (*pb.SearchOnecProjectResp, error) {
	if in.Page == 0 {
		in.Page = vars.Page
	}
	if in.PageSize == 0 {
		in.PageSize = vars.PageSize
	}

	// 构建查询条件
	var queryStr string
	var args []interface{}

	if in.Name != "" {
		queryStr = "name LIKE ?"
		args = append(args, "%"+in.Name+"%")
	}

	if in.Uuid != "" {
		if queryStr != "" {
			queryStr += " AND "
		}
		queryStr += "uuid = ?"
		args = append(args, in.Uuid)
	}

	// 执行搜索
	projects, total, err := l.svcCtx.OnecProjectModel.Search(l.ctx, in.OrderStr, in.IsAsc, in.Page, in.PageSize, queryStr, args...)
	if err != nil {
		// 修复：检查是否为 ErrNotFound，如果是则返回空结果而不是错误
		if errors.Is(err, model.ErrNotFound) {
			return &pb.SearchOnecProjectResp{
				Data:  []*pb.ProjectInfo{},
				Total: 0,
			}, nil
		}

		l.Errorf("搜索项目失败，错误: %v", err)
		return nil, errorx.Msg("搜索项目失败")
	}

	// 转换结果
	var data []*pb.ProjectInfo
	for _, p := range projects {
		statistics, err := l.svcCtx.OnecProjectModel.GetProjectStatistics(l.ctx, p.Id)
		if err != nil {
			l.Errorf("获取项目统计信息失败，错误: %v", err)
		}

		data = append(data, &pb.ProjectInfo{
			Id:            p.Id,
			Name:          p.Name,
			Uuid:          p.Uuid,
			Description:   p.Description,
			CreatedBy:     p.CreatedBy,
			UpdatedBy:     p.UpdatedBy,
			AdminCount:    statistics.AdminCount,
			ResourceCount: statistics.ProjectClusterCount,
			CreatedAt:     p.CreatedAt.Unix(),
			UpdatedAt:     p.UpdatedAt.Unix(),
			IsSystem:      p.IsSystem,
		})
	}

	return &pb.SearchOnecProjectResp{
		Data:  data,
		Total: total,
	}, nil
}
