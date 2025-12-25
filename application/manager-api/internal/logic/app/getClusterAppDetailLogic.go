package app

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/manager-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/client/managerservice"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetClusterAppDetailLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 根据应用ID获取应用的详细配置信息
func NewGetClusterAppDetailLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetClusterAppDetailLogic {
	return &GetClusterAppDetailLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetClusterAppDetailLogic) GetClusterAppDetail(req *types.ClusterAppDetailRequest) (resp *types.ClusterAppDetail, err error) {
	rpcReq := &managerservice.ClusterAppDetailReq{
		Id: uint64(req.Id),
	}

	rpcResp, err := l.svcCtx.ManagerRpc.AppDetail(l.ctx, rpcReq)
	if err != nil {
		l.Errorf("调用RPC服务获取应用详情失败，ID: %d, error: %v", req.Id, err)
		return nil, fmt.Errorf("获取应用详情失败: %w", err)
	}
	// 5. 转换响应数据
	resp = &types.ClusterAppDetail{
		Id:                 uint64(rpcResp.Id),
		ClusterUuid:        rpcResp.ClusterUuid,
		AppName:            rpcResp.AppName,
		AppCode:            rpcResp.AppCode,
		AppType:            rpcResp.AppType,
		IsDefault:          rpcResp.IsDefault,
		AppUrl:             rpcResp.AppUrl,
		Port:               rpcResp.Port,
		Protocol:           rpcResp.Protocol,
		AuthEnabled:        rpcResp.AuthEnabled,
		AuthType:           rpcResp.AuthType,
		Username:           rpcResp.Username,
		Password:           rpcResp.Password, // 不返回密码明文
		Token:              rpcResp.Token,    // 不返回Token明文
		AccessKey:          rpcResp.AccessKey,
		AccessSecret:       rpcResp.AccessSecret, // 不返回Secret明文
		TlsEnabled:         rpcResp.TlsEnabled,
		CaFile:             rpcResp.CaFile,
		CaKey:              rpcResp.CaKey, // 不返回密钥明文
		CaCert:             rpcResp.CaCert,
		ClientCert:         rpcResp.ClientCert,
		ClientKey:          rpcResp.ClientKey, // 不返回密钥明文
		InsecureSkipVerify: rpcResp.InsecureSkipVerify,
		Status:             rpcResp.Status,
		CreatedBy:          rpcResp.CreatedBy,
		UpdatedBy:          rpcResp.UpdatedBy,
		CreatedAt:          rpcResp.CreatedAt,
		UpdatedAt:          rpcResp.UpdatedAt,
	}
	return resp, nil
}
