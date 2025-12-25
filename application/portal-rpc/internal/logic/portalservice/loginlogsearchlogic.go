package portalservicelogic

import (
	"context"
	"errors"
	"strings"

	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"
	"github.com/yanshicheng/kube-nova/common/vars"
	"github.com/yanshicheng/kube-nova/pkg/utils"

	"github.com/zeromicro/go-zero/core/logx"
)

type LoginLogSearchLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewLoginLogSearchLogic(ctx context.Context, svcCtx *svc.ServiceContext) *LoginLogSearchLogic {
	return &LoginLogSearchLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// LoginLogSearch 搜索登录日志列表
func (l *LoginLogSearchLogic) LoginLogSearch(in *pb.SearchSysLoginLogReq) (*pb.SearchSysLoginLogResp, error) {

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

	// 用户名模糊查询
	if in.Username != "" {
		conditions = append(conditions, "`username` LIKE ? AND")
		args = append(args, "%"+strings.TrimSpace(in.Username)+"%")
	}

	// IP地址模糊查询
	if in.IpAddress != "" {
		conditions = append(conditions, "`ip_address` LIKE ? AND")
		args = append(args, "%"+strings.TrimSpace(in.IpAddress)+"%")
	}

	// 用户代理模糊查询
	if in.UserAgent != "" {
		conditions = append(conditions, "`user_agent` LIKE ? AND")
		args = append(args, "%"+strings.TrimSpace(in.UserAgent)+"%")
	}

	// 去掉最后一个 " AND "，避免 SQL 语法错误
	query := utils.RemoveQueryADN(conditions)

	// 执行分页查询
	loginLogList, total, err := l.svcCtx.SysLoginLog.Search(l.ctx, in.OrderField, in.IsAsc, in.Page, in.PageSize, query, args...)
	if err != nil && !errors.Is(err, model.ErrNotFound) {
		l.Errorf("查询登录日志列表失败: %v", err)
		return nil, errorx.Msg("查询登录日志列表失败")
	}

	// 转换结果
	pbLoginLogs := l.convertToSearchResult(loginLogList)

	return &pb.SearchSysLoginLogResp{
		Data:  pbLoginLogs,
		Total: total,
	}, nil
}

// convertToSearchResult 转换搜索结果
func (l *LoginLogSearchLogic) convertToSearchResult(loginLogList []*model.SysLoginLog) []*pb.SysLoginLog {
	var pbLoginLogs []*pb.SysLoginLog
	for _, loginLog := range loginLogList {
		pbLoginLog := &pb.SysLoginLog{
			Id:          loginLog.Id,
			UserId:      loginLog.UserId,
			Username:    loginLog.Username,
			LoginStatus: loginLog.LoginStatus,
			IpAddress:   loginLog.IpAddress,
			UserAgent:   loginLog.UserAgent,
			LoginTime:   loginLog.LoginTime.Unix(),
		}
		pbLoginLogs = append(pbLoginLogs, pbLoginLog)
	}
	return pbLoginLogs
}
