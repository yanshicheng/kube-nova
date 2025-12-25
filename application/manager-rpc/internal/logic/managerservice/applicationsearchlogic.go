package managerservicelogic

import (
	"context"
	"errors"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/zeromicro/go-zero/core/logx"
)

type ApplicationSearchLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewApplicationSearchLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ApplicationSearchLogic {
	return &ApplicationSearchLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ApplicationSearch 查询应用列表
func (l *ApplicationSearchLogic) ApplicationSearch(in *pb.SearchOnecProjectApplicationReq) (*pb.SearchOnecProjectApplicationResp, error) {
	// 参数校验
	if in.WorkspaceId == 0 {
		l.Errorf("参数校验失败: workspaceId 不能为空")
		return nil, status.Error(codes.InvalidArgument, "workspaceId 不能为空")
	}

	// 构建查询条件
	var queryStr string
	var args []interface{}

	queryStr = "`workspace_id` = ?"
	args = append(args, in.WorkspaceId)

	// 如果指定了服务中文名，添加模糊查询
	if in.NameCn != "" {
		queryStr += " AND `name_cn` LIKE ?"
		args = append(args, fmt.Sprintf("%%%s%%", in.NameCn))
	}

	// 查询数据
	applications, err := l.svcCtx.OnecProjectApplication.SearchNoPage(
		l.ctx, "", true, queryStr, args...)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			l.Infof("未查询到应用: workspaceId=%d", in.WorkspaceId)
			return &pb.SearchOnecProjectApplicationResp{Data: []*pb.OnecProjectApplication{}}, nil
		}
		l.Errorf("查询应用失败: %v, workspaceId=%d", err, in.WorkspaceId)
		return nil, status.Error(codes.Internal, "查询应用失败")
	}

	// 转换数据格式
	var pbApplications []*pb.OnecProjectApplication
	for _, app := range applications {
		pbApplications = append(pbApplications, &pb.OnecProjectApplication{
			Id:           app.Id,
			WorkspaceId:  app.WorkspaceId,
			NameCn:       app.NameCn,
			NameEn:       app.NameEn,
			ResourceType: app.ResourceType,
			Description:  app.Description,
			CreatedBy:    app.CreatedBy,
			UpdatedBy:    app.UpdatedBy,
			CreatedAt:    app.CreatedAt.Unix(),
			UpdatedAt:    app.UpdatedAt.Unix(),
		})
	}

	return &pb.SearchOnecProjectApplicationResp{
		Data: pbApplications,
	}, nil
}
