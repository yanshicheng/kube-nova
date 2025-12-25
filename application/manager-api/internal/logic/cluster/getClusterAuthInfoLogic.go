package cluster

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/manager-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetClusterAuthInfoLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetClusterAuthInfoLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetClusterAuthInfoLogic {
	return &GetClusterAuthInfoLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetClusterAuthInfoLogic) GetClusterAuthInfo(req *types.DefaultIdRequest) (resp *types.ClusterAuthInfo, err error) {

	// 先获取集群详情，获取UUID
	clusterResp, err := l.svcCtx.ManagerRpc.ClusterDetail(l.ctx, &pb.ClusterDetailReq{
		Id: req.Id,
	})
	if err != nil {
		l.Errorf("RPC调用获取集群详情失败: %v", err)
		return nil, fmt.Errorf("获取集群详情失败: %w", err)
	}

	// 调用RPC获取集群认证信息
	rpcResp, err := l.svcCtx.ManagerRpc.ClusterAuthInfo(l.ctx, &pb.ClusterAuthInfoReq{
		ClusterUuid: clusterResp.Uuid,
	})
	if err != nil {
		l.Errorf("RPC调用获取集群认证信息失败: %v", err)
		return nil, fmt.Errorf("获取集群认证信息失败: %w", err)
	}

	// 转换响应
	resp = &types.ClusterAuthInfo{
		Id:                 rpcResp.Id,
		ClusterUuid:        rpcResp.ClusterUuid,
		AuthType:           rpcResp.AuthType,
		ApiServerHost:      rpcResp.ApiServerHost,
		KubeFile:           rpcResp.KubeFile,
		Token:              rpcResp.Token,
		CaCert:             rpcResp.CaCert,
		CaFile:             rpcResp.CaFile,
		ClientCert:         rpcResp.ClientCert,
		CertFile:           rpcResp.CertFile,
		ClientKey:          rpcResp.ClientKey,
		KeyFile:            rpcResp.KeyFile,
		InsecureSkipVerify: rpcResp.InsecureSkipVerify,
		CreatedAt:          rpcResp.CreatedAt,
		UpdatedAt:          rpcResp.UpdatedAt,
	}

	return resp, nil
}
