package managerservicelogic

import (
	"context"
	"errors"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"
	"github.com/zeromicro/go-zero/core/logx"
)

type VersionSearchLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewVersionSearchLogic(ctx context.Context, svcCtx *svc.ServiceContext) *VersionSearchLogic {
	return &VersionSearchLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// VersionSearch 查询版本列表
func (l *VersionSearchLogic) VersionSearch(in *pb.SearchOnecProjectVersionReq) (*pb.SearchOnecProjectVersionResp, error) {
	// 参数校验
	if in.ApplicationId == 0 {
		l.Errorf("参数校验失败: applicationId 不能为空")
		return nil, errorx.Msg("应用ID不能为空")
	}

	// 验证应用是否存在
	_, err := l.svcCtx.OnecProjectApplication.FindOne(l.ctx, in.ApplicationId)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			l.Errorf("应用不存在: applicationId=%d", in.ApplicationId)
			return nil, errorx.Msg("应用不存在")
		}
		l.Errorf("查询应用失败: %v, applicationId=%d", err, in.ApplicationId)
		return nil, errorx.Msg("查询应用失败")
	}

	// 构建查询条件
	queryStr := "`application_id` = ?"
	args := []interface{}{in.ApplicationId}

	// 查询数据
	versions, err := l.svcCtx.OnecProjectVersion.SearchNoPage(
		l.ctx, "created_at", false, queryStr, args...)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			l.Infof("未查询到版本: applicationId=%d", in.ApplicationId)
			return &pb.SearchOnecProjectVersionResp{Data: []*pb.OnecProjectVersion{}}, nil
		}
		l.Errorf("查询版本失败: %v, applicationId=%d", err, in.ApplicationId)
		return nil, errorx.Msg("查询版本失败")
	}

	// 转换数据格式
	var pbVersions []*pb.OnecProjectVersion
	for _, ver := range versions {
		// 解析 JSON 标签
		pbVersions = append(pbVersions, &pb.OnecProjectVersion{
			Id:            ver.Id,
			ApplicationId: ver.ApplicationId,
			Version:       ver.Version,
			ResourceName:  ver.ResourceName,
			CreatedBy:     ver.CreatedBy,
			UpdatedBy:     ver.UpdatedBy,
			CreatedAt:     ver.CreatedAt.Unix(),
			UpdatedAt:     ver.UpdatedAt.Unix(),
			Status:        ver.Status,
			ParentAppName: ver.ParentAppName,
			VersionRole:   ver.VersionRole,
		})
	}

	return &pb.SearchOnecProjectVersionResp{
		Data: pbVersions,
	}, nil
}
