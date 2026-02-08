package repositoryservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-rpc/internal/repositorymanager/types"
	"github.com/yanshicheng/kube-nova/application/console-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListRetentionExecutionsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListRetentionExecutionsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListRetentionExecutionsLogic {
	return &ListRetentionExecutionsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ListRetentionExecutionsLogic) ListRetentionExecutions(in *pb.ListRetentionExecutionsReq) (*pb.ListRetentionExecutionsResp, error) {
	l.Infof("查询保留策略执行历史: registryUuid=%s, policyId=%d, page=%d, pageSize=%d",
		in.RegistryUuid, in.PolicyId, in.Page, in.PageSize)

	if in.PolicyId <= 0 {
		l.Errorf("查询执行历史失败: PolicyId 无效 (policyId=%d)", in.PolicyId)
		return nil, errorx.Msg("策略ID无效，请先配置保留策略")
	}

	client, err := l.svcCtx.HarborManager.Get(in.RegistryUuid)
	if err != nil {
		l.Errorf("获取仓库客户端失败: %v", err)
		return nil, errorx.Msg("获取仓库客户端失败")
	}

	req := types.ListRequest{
		Page:     in.Page,
		PageSize: in.PageSize,
	}

	resp, err := client.Retention().ListExecutions(in.PolicyId, req)
	if err != nil {
		l.Errorf("查询保留策略执行历史失败: %v", err)
		return nil, errorx.Msg("查询保留策略执行历史失败")
	}

	// 转换执行记录
	var items []*pb.RetentionExecution
	if resp.Items != nil {
		for _, exec := range resp.Items {
			dryRun := int32(0)
			if exec.DryRun {
				dryRun = 1
			}

			items = append(items, &pb.RetentionExecution{
				Id:        exec.ID,
				PolicyId:  exec.PolicyID,
				Status:    exec.Status,
				Trigger:   exec.Trigger,
				StartTime: exec.StartTime.Unix(),
				EndTime:   exec.EndTime.Unix(),
				DryRun:    dryRun,
			})
		}
	}

	l.Infof("查询执行历史成功: policyId=%d, total=%d, returned=%d",
		in.PolicyId, resp.Total, len(items))

	return &pb.ListRetentionExecutionsResp{
		Items:      items,
		Total:      int64(resp.Total),
		Page:       in.Page,
		PageSize:   in.PageSize,
		TotalPages: int64(resp.TotalPages),
	}, nil
}
