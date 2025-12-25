package portalservicelogic

import (
	"context"
	"errors"
	"strings"

	"github.com/yanshicheng/kube-nova/common/handler/errorx"
	"github.com/yanshicheng/kube-nova/common/vars"
	"github.com/yanshicheng/kube-nova/pkg/utils"

	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type APISearchLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAPISearchLogic(ctx context.Context, svcCtx *svc.ServiceContext) *APISearchLogic {
	return &APISearchLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// APISearch 搜索系统API列表
func (l *APISearchLogic) APISearch(in *pb.SearchSysAPIReq) (*pb.SearchSysAPIResp, error) {
	// 设置默认参数
	if in.Page <= 0 {
		in.Page = vars.Page
	}
	if in.PageSize <= 0 {
		in.PageSize = vars.PageSize
	}
	if in.OrderField == "" {
		in.OrderField = vars.OrderField
	}

	// 构建查询条件
	var conditions []string
	var args []interface{}

	// 父级ID条件
	if in.ParentId > 0 {
		conditions = append(conditions, "`parent_id` = ? AND")
		args = append(args, in.ParentId)
	}

	// API名称模糊查询
	if in.Name != "" {
		conditions = append(conditions, "`name` LIKE ? AND")
		args = append(args, "%"+strings.TrimSpace(in.Name)+"%")
	}

	// API路径模糊查询
	if in.Path != "" {
		conditions = append(conditions, "`path` LIKE ? AND")
		args = append(args, "%"+strings.TrimSpace(in.Path)+"%")
	}

	// HTTP方法精确查询
	if in.Method != "" {
		conditions = append(conditions, "`method` = ? AND")
		args = append(args, strings.ToUpper(strings.TrimSpace(in.Method)))
	}

	// 权限类型条件
	conditions = append(conditions, "`is_permission` = ? AND")
	is_permission := 1
	args = append(args, is_permission)

	// 去掉最后一个 " AND "，避免 SQL 语法错误
	query := utils.RemoveQueryADN(conditions)

	// 执行分页查询
	apiList, total, err := l.svcCtx.SysApi.Search(l.ctx, in.OrderField, in.IsAsc, in.Page, in.PageSize, query, args...)
	if err != nil && !errors.Is(err, model.ErrNotFound) {
		l.Errorf("查询API列表失败: %v", err)
		return nil, errorx.Msg("查询API列表失败")
	}

	// 转换结果
	pbAPIs := l.convertToSearchResult(apiList)

	return &pb.SearchSysAPIResp{
		Data:  pbAPIs,
		Total: total,
	}, nil
}

// convertToSearchResult 转换搜索结果
func (l *APISearchLogic) convertToSearchResult(apiList []*model.SysApi) []*pb.SysAPI {
	var pbAPIs []*pb.SysAPI
	for _, api := range apiList {
		pbAPI := &pb.SysAPI{
			Id:           api.Id,
			ParentId:     api.ParentId,
			Name:         api.Name,
			Path:         api.Path,
			Method:       api.Method,
			IsPermission: api.IsPermission,
			CreatedBy:    api.CreatedBy,
			UpdatedBy:    api.UpdatedBy,
			CreatedAt:    api.CreatedAt.Unix(),
			UpdatedAt:    api.UpdatedAt.Unix(),
		}
		pbAPIs = append(pbAPIs, pbAPI)
	}
	return pbAPIs
}
