package loginLog

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/portal-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type SearchSysLoginLogLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewSearchSysLoginLogLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SearchSysLoginLogLogic {
	return &SearchSysLoginLogLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SearchSysLoginLogLogic) SearchSysLoginLog(req *types.SearchSysLoginLogRequest) (resp *types.SearchSysLoginLogResponse, err error) {

	// 调用 RPC 服务搜索登录日志
	res, err := l.svcCtx.PortalRpc.LoginLogSearch(l.ctx, &pb.SearchSysLoginLogReq{
		Page:       req.Page,
		PageSize:   req.PageSize,
		OrderField: req.OrderField,
		IsAsc:      req.IsAsc,
		UserId:     req.UserId,
		Username:   req.Username,
		IpAddress:  req.IpAddress,
		UserAgent:  req.UserAgent,
	})
	if err != nil {
		l.Errorf("搜索登录日志失败: error=%v", err)
		return nil, err
	}

	// 转换登录日志列表
	var logs []types.SysLoginLog
	for _, log := range res.Data {
		logs = append(logs, types.SysLoginLog{
			Id:          log.Id,
			UserId:      log.UserId,
			Username:    log.Username,
			LoginStatus: log.LoginStatus,
			IpAddress:   log.IpAddress,
			UserAgent:   log.UserAgent,
			LoginTime:   log.LoginTime,
		})
	}

	return &types.SearchSysLoginLogResponse{
		Items: logs,
		Total: res.Total,
	}, nil
}
