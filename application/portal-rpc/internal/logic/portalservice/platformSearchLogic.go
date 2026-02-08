package portalservicelogic

import (
	"context"
	"strings"

	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type PlatformSearchLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewPlatformSearchLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PlatformSearchLogic {
	return &PlatformSearchLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// PlatformSearch 搜索平台（支持分页）
func (l *PlatformSearchLogic) PlatformSearch(in *pb.SearchSysPlatformReq) (*pb.SearchSysPlatformResp, error) {
	// 设置默认分页参数
	page := in.Page
	if page < 1 {
		page = 1
	}
	pageSize := in.PageSize
	if pageSize < 1 {
		pageSize = 10
	}

	// 构建查询条件
	var conditions []string
	var args []interface{}

	if in.PlatformCode != "" {
		conditions = append(conditions, "`platform_code` LIKE ?")
		args = append(args, "%"+in.PlatformCode+"%")
	}
	if in.PlatformName != "" {
		conditions = append(conditions, "`platform_name` LIKE ?")
		args = append(args, "%"+in.PlatformName+"%")
	}

	queryStr := strings.Join(conditions, " AND ")

	// 设置排序字段
	orderStr := in.OrderField
	if orderStr == "" {
		orderStr = "sort"
	}

	// 查询数据
	platforms, total, err := l.svcCtx.SysPlatformModel.Search(l.ctx, orderStr, in.IsAsc, page, pageSize, queryStr, args...)
	if err != nil {
		l.Errorf("搜索平台失败: %v", err)
		return nil, errorx.Msg("搜索平台失败")
	}

	// 转换为 protobuf 格式
	var pbPlatforms []*pb.SysPlatform
	for _, platform := range platforms {
		pbPlatform := &pb.SysPlatform{
			Id:           platform.Id,
			PlatformCode: platform.PlatformCode,
			PlatformName: platform.PlatformName,
			PlatformDesc: platform.PlatformDesc,
			PlatformIcon: platform.PlatformIcon,
			Sort:         platform.Sort,
			IsEnable:     platform.IsEnable,
			IsDefault:    platform.IsDefault,
			CreateTime:   platform.CreateTime.Unix(),
			UpdateTime:   platform.UpdateTime.Unix(),
			CreateBy:     platform.CreateBy.String,
			UpdateBy:     platform.UpdateBy.String,
		}
		pbPlatforms = append(pbPlatforms, pbPlatform)
	}

	l.Infof("搜索平台成功，共找到 %d 条记录", total)
	return &pb.SearchSysPlatformResp{
		Data:  pbPlatforms,
		Total: total,
	}, nil
}
