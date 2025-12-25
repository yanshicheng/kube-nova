package managerservicelogic

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"
	"github.com/yanshicheng/kube-nova/common/k8smanager/cluster"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"github.com/zeromicro/go-zero/core/utils"

	utils2 "github.com/yanshicheng/kube-nova/common/utils"
	"github.com/zeromicro/go-zero/core/logx"
)

type ClusterAddLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewClusterAddLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ClusterAddLogic {
	return &ClusterAddLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ClusterAddLogic) ClusterAdd(in *pb.AddClusterRequest) (*pb.AddClusterResponse, error) {
	// 创建默认的uuid
	uuid := utils.NewUuid()

	// 判断 AuthType 是不是 incluster，certificate，token，kubeconfig 如果不是需要报错
	authtypeList := []string{"incluster", "certificate", "token", "kubeconfig"}
	if !IsInList(in.AuthType, authtypeList) {
		l.Errorf("不支持的认证类型: %s", in.AuthType)
		return nil, errorx.Msg("不支持的认证类型!")
	}

	// 判断集群连接是否正常
	if err := l.IsClusterConnectOk(in, uuid); err != nil {
		l.Errorf("集群连接测试失败: %v", err)
		return nil, errorx.Msg("集群连接测试失败")
	}

	nodeLb := ""
	var err error
	if in.NodeLb != "" {
		nodeLb, err = utils2.ValidateAndCleanAddresses(in.NodeLb)
		if err != nil {
			l.Errorf("集群节点LB地址格式错误: %v", err)
			return nil, errorx.Msg("集群节点LB地址格式错误")
		}
	}
	masterLb := ""
	if in.MasterLb != "" {
		masterLb, err = utils2.ValidateAndCleanAddresses(in.MasterLb)
		if err != nil {
			l.Errorf("集群节点LB地址格式错误: %v", err)
			return nil, errorx.Msg("集群节点LB地址格式错误")
		}
	}
	ingressDomain := ""
	if in.IngressDomain != "" {
		ingressDomain, err = utils2.ValidateAndCleanDomainSuffixes(in.IngressDomain)
		if err != nil {
			l.Errorf("集群节点LB地址格式错误: %v", err)
			return nil, errorx.Msg("集群节点LB地址格式错误")
		}
	}

	// 使用事务确保数据一致性
	err = l.svcCtx.OnecClusterModel.TransCtx(l.ctx, func(ctx context.Context, session sqlx.Session) error {
		// 1. 基本信息保存到数据库，初始状态设置为 1（同步中）
		result, err := session.ExecCtx(ctx,
			`INSERT INTO onec_cluster (name, avatar, description, datacenter, cluster_type, 
             uuid, environment, region, zone, provider, is_managed, node_lb, master_lb, 
             created_by, updated_by, status, is_deleted,ingress_domain) 
             VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?,?)`,
			in.Name,
			in.Avatar,
			in.Description,
			in.Datacenter,
			in.ClusterType,
			uuid,
			in.Environment,
			in.Region,
			in.Zone,
			in.Provider,
			in.IsManaged,
			nodeLb,
			masterLb,
			in.CreatedBy,
			in.UpdatedBy,
			1, // status 初始为 1（同步中）
			0, // is_deleted 默认为0
			ingressDomain,
		)
		if err != nil {
			l.Errorf("插入集群基本信息失败: %v", err)
			return errorx.Msg("添加集群基本信息失败")
		}

		// 检查是否成功插入
		if rowsAffected, _ := result.RowsAffected(); rowsAffected == 0 {
			return errorx.Msg("添加集群基本信息失败: 未插入任何记录")
		}

		// 2. 认证信息保存到认证表中
		// 注意：token, ca_file, cert_file, key_file 在 Model 中是 string 类型，直接传字符串
		// ca_cert, client_cert, client_key, kubefile 在 Model 中是 sql.NullString 类型，需要包装
		result, err = session.ExecCtx(ctx,
			`INSERT INTO onec_cluster_auth (cluster_uuid, auth_type, api_server_host, 
             token, ca_cert, ca_file, client_cert, cert_file, 
             client_key, key_file, insecure_skip_verify, kubefile) 
             VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			uuid,
			in.AuthType,
			in.ApiServerHost,
			in.Token, // string 类型，直接传
			sql.NullString{String: in.CaCert, Valid: in.CaCert != ""}, // NullString 类型
			in.CaFile, // string 类型，直接传
			sql.NullString{String: in.ClientCert, Valid: in.ClientCert != ""}, // NullString 类型
			in.CertFile, // string 类型，直接传
			sql.NullString{String: in.ClientKey, Valid: in.ClientKey != ""}, // NullString 类型
			in.KeyFile, // string 类型，直接传（修复点：之前错误使用了 NullString）
			in.InsecureSkipVerify,
			sql.NullString{String: in.KubeFile, Valid: in.KubeFile != ""}, // NullString 类型
		)
		if err != nil {
			l.Errorf("插入集群认证信息失败: %v", err)
			return errorx.Msg("添加集群认证信息失败")
		}

		// 检查是否成功插入
		if rowsAffected, _ := result.RowsAffected(); rowsAffected == 0 {
			return errorx.Msg("添加集群认证信息失败: 未插入任何记录")
		}

		return nil
	})

	if err != nil {
		l.Errorf("添加集群失败（事务回滚）: %v", err)
		return nil, err
	}

	// 异步执行集群同步操作
	go func() {
		// 使用新的 context，避免父 context 取消影响异步操作
		syncCtx := context.Background()

		if err := l.svcCtx.SyncOperator.SyncOneCLuster(syncCtx, uuid, in.CreatedBy, true); err != nil {
			l.Errorf("集群同步失败: %v", err)
		} else {
			l.Infof("集群同步成功")
		}
	}()

	return &pb.AddClusterResponse{}, nil
}

// updateClusterStatus 更新集群状态
func (l *ClusterAddLogic) updateClusterStatus(ctx context.Context, uuid string, status int64) error {
	// 先查询集群获取 ID
	cluster, err := l.svcCtx.OnecClusterModel.FindOneByUuid(ctx, uuid)
	if err != nil {
		return err
	}

	// 更新状态
	cluster.Status = status
	return l.svcCtx.OnecClusterModel.Update(ctx, cluster)
}

// IsClusterConnectOk 判断集群连接是否正常
func (l *ClusterAddLogic) IsClusterConnectOk(in *pb.AddClusterRequest, clusterUuid string) error {
	l.Infof("开始测试集群连接，临时集群 UUID: %s", clusterUuid)

	// 构建集群配置
	clusterConfig := cluster.Config{
		Name:     clusterUuid,
		ID:       clusterUuid,
		AuthType: cluster.AuthType(in.AuthType),
	}

	// 根据认证类型设置配置
	switch in.AuthType {
	case "kubeconfig":
		if in.KubeFile == "" {
			return errorx.Msg("kubeconfig 内容不能为空")
		}
		clusterConfig.KubeConfigData = in.KubeFile
		l.Info("使用 kubeconfig 认证")

	case "token":
		if in.ApiServerHost == "" {
			return errorx.Msg("API Server 地址不能为空")
		}
		if in.Token == "" {
			return errorx.Msg("Token 不能为空")
		}
		clusterConfig.APIServer = in.ApiServerHost
		clusterConfig.Token = in.Token
		clusterConfig.Insecure = in.InsecureSkipVerify == 1
		l.Infof("使用 Token 认证 - API Server: %s, Token长度: %d",
			in.ApiServerHost, len(in.Token))

	case "certificate":
		if in.ApiServerHost == "" {
			return errorx.Msg("API Server 地址不能为空")
		}
		if in.CertFile == "" || in.KeyFile == "" {
			return errorx.Msg("证书文件和密钥文件不能为空")
		}
		clusterConfig.APIServer = in.ApiServerHost
		clusterConfig.CertFile = in.CertFile
		clusterConfig.KeyFile = in.KeyFile
		clusterConfig.CAFile = in.CaFile
		clusterConfig.Insecure = in.InsecureSkipVerify == 1
		l.Info("使用证书认证")

	case "incluster":
		l.Info("使用 InCluster 认证")

	default:
		return errorx.Msg("不支持的认证模式")
	}

	// 添加临时集群到管理器
	l.Info("添加临时集群到管理器")
	if err := l.svcCtx.K8sManager.AddCluster(clusterConfig); err != nil {
		l.Errorf("添加临时集群失败: %v", err)
		return fmt.Errorf("添加临时集群失败: %w", err)
	}
	l.Info("临时集群添加成功")

	// 使用 defer 确保清理临时集群
	defer func() {
		l.Infof("删除临时测试集群: %s", clusterUuid)
		if err := l.svcCtx.K8sManager.RemoveCluster(clusterUuid); err != nil {
			l.Errorf("删除临时集群失败: %v", err)
		} else {
			l.Info("临时集群删除成功")
		}
	}()

	client, err := l.svcCtx.K8sManager.GetCluster(l.ctx, clusterUuid)
	if err != nil {
		l.Errorf("获取集群客户端失败: %v", err)
		return fmt.Errorf("获取集群客户端失败: %w", err)
	}

	// 执行健康检查
	l.Info("开始健康检查")
	if err := client.HealthCheck(l.ctx); err != nil {
		l.Errorf("集群健康检查失败: %v", err)
		return fmt.Errorf("集群健康检查失败: %w", err)
	}

	l.Info("集群连接测试成功")
	return nil
}
