package managerservicelogic

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/model"
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

	// ==================== 1. 参数校验 ====================
	// 判断 AuthType 是不是 incluster，certificate，token，kubeconfig 如果不是需要报错
	authtypeList := []string{"incluster", "certificate", "token", "kubeconfig"}
	if !IsInList(in.AuthType, authtypeList) {
		l.Errorf("不支持的认证类型: %s", in.AuthType)
		return nil, errorx.Msg("不支持的认证类型!")
	}

	// 校验价格配置ID（必填）
	if in.PriceConfigId == 0 {
		l.Errorf("价格配置ID不能为空")
		return nil, errorx.Msg("价格配置ID不能为空")
	}

	// 校验 Prometheus 配置（如果启用）
	if in.EnablePrometheus {
		if in.PrometheusUrl == "" {
			l.Errorf("启用 Prometheus 时，地址不能为空")
			return nil, errorx.Msg("Prometheus 地址不能为空")
		}
		if in.PrometheusPort <= 0 || in.PrometheusPort > 65535 {
			l.Errorf("Prometheus 端口无效: %d", in.PrometheusPort)
			return nil, errorx.Msg("Prometheus 端口无效")
		}
		if in.PrometheusProtocol == "" {
			in.PrometheusProtocol = "http" // 默认使用 http
		}
		if in.PrometheusAuthType == "" {
			in.PrometheusAuthType = "none" // 默认无认证
		}
	}

	// ==================== 2. 预检查 ====================
	// 判断集群连接是否正常
	if err := l.IsClusterConnectOk(in, uuid); err != nil {
		l.Errorf("集群连接测试失败: %v", err)
		return nil, errorx.Msg("集群连接测试失败")
	}

	// 检查价格配置是否存在
	_, err := l.svcCtx.OnecBillingPriceConfigModel.FindOne(l.ctx, in.PriceConfigId)
	if err != nil {
		if err == model.ErrNotFound {
			l.Errorf("价格配置不存在, 配置ID: %d", in.PriceConfigId)
			return nil, errorx.Msg("指定的价格配置不存在")
		}
		l.Errorf("查询价格配置失败: %v", err)
		return nil, errorx.Msg("查询价格配置失败")
	}
	l.Infof("价格配置验证通过, 配置ID: %d", in.PriceConfigId)

	// ==================== 3. 地址格式校验 ====================
	nodeLb := ""
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

	// ==================== 4. 处理计费开始时间 ====================
	var billingStartTime sql.NullTime
	if in.BillingStartTime > 0 {
		billingStartTime = sql.NullTime{
			Time:  time.Unix(in.BillingStartTime, 0),
			Valid: true,
		}
		l.Infof("使用指定的计费开始时间: %v", billingStartTime.Time)
	} else {
		billingStartTime = sql.NullTime{
			Time:  time.Now(),
			Valid: true,
		}
		l.Infof("使用当前时间作为计费开始时间: %v", billingStartTime.Time)
	}

	// ==================== 5. 使用事务确保数据一致性 ====================
	err = l.svcCtx.OnecClusterModel.TransCtx(l.ctx, func(ctx context.Context, session sqlx.Session) error {
		// 5.1 基本信息保存到数据库，初始状态设置为 1（同步中）
		l.Infof("步骤5.1: 插入集群基本信息, 集群名称: %s, UUID: %s", in.Name, uuid)
		result, err := session.ExecCtx(ctx,
			`INSERT INTO onec_cluster (name, avatar, description, datacenter, cluster_type, 
             uuid, environment, region, zone, provider, is_managed, node_lb, master_lb, 
             created_by, updated_by, status, is_deleted, ingress_domain) 
             VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
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

		if rowsAffected, _ := result.RowsAffected(); rowsAffected == 0 {
			return errorx.Msg("添加集群基本信息失败: 未插入任何记录")
		}
		l.Infof("集群基本信息插入成功")

		// 5.2 认证信息保存到认证表中
		l.Infof("步骤5.2: 插入集群认证信息")
		result, err = session.ExecCtx(ctx,
			`INSERT INTO onec_cluster_auth (cluster_uuid, auth_type, api_server_host, 
             token, ca_cert, ca_file, client_cert, cert_file, 
             client_key, key_file, insecure_skip_verify, kubefile) 
             VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			uuid,
			in.AuthType,
			in.ApiServerHost,
			in.Token,
			sql.NullString{String: in.CaCert, Valid: in.CaCert != ""},
			in.CaFile,
			sql.NullString{String: in.ClientCert, Valid: in.ClientCert != ""},
			in.CertFile,
			sql.NullString{String: in.ClientKey, Valid: in.ClientKey != ""},
			in.KeyFile,
			in.InsecureSkipVerify,
			sql.NullString{String: in.KubeFile, Valid: in.KubeFile != ""},
		)
		if err != nil {
			l.Errorf("插入集群认证信息失败: %v", err)
			return errorx.Msg("添加集群认证信息失败")
		}

		if rowsAffected, _ := result.RowsAffected(); rowsAffected == 0 {
			return errorx.Msg("添加集群认证信息失败: 未插入任何记录")
		}
		l.Infof("集群认证信息插入成功")

		// 5.3 插入费用配置绑定
		l.Infof("步骤5.3: 插入费用配置绑定, 价格配置ID: %d, 集群UUID: %s", in.PriceConfigId, uuid)
		result, err = session.ExecCtx(ctx,
			`INSERT INTO onec_billing_config_binding (binding_type, binding_cluster_uuid, 
             binding_project_id, price_config_id, billing_start_time, 
             created_by, updated_by, is_deleted) 
             VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			"cluster",
			uuid,
			0,
			in.PriceConfigId,
			billingStartTime,
			in.CreatedBy,
			in.CreatedBy,
			0,
		)
		if err != nil {
			l.Errorf("插入费用配置绑定失败: %v", err)
			return errorx.Msg("添加费用配置绑定失败")
		}

		if rowsAffected, _ := result.RowsAffected(); rowsAffected == 0 {
			return errorx.Msg("添加费用配置绑定失败: 未插入任何记录")
		}
		l.Infof("费用配置绑定插入成功")

		// 5.4 如果启用了 Prometheus，插入应用配置
		if in.EnablePrometheus {
			l.Infof("步骤5.4: 插入 Prometheus 应用配置, 地址: %s:%d", in.PrometheusUrl, in.PrometheusPort)

			// 构建可选字段
			var prometheusToken, prometheusCaCert, prometheusClientCert, prometheusClientKey sql.NullString

			if in.PrometheusToken != "" {
				prometheusToken = sql.NullString{String: in.PrometheusToken, Valid: true}
			}
			if in.PrometheusCaCert != "" {
				prometheusCaCert = sql.NullString{String: in.PrometheusCaCert, Valid: true}
			}
			if in.PrometheusClientCert != "" {
				prometheusClientCert = sql.NullString{String: in.PrometheusClientCert, Valid: true}
			}
			if in.PrometheusClientKey != "" {
				prometheusClientKey = sql.NullString{String: in.PrometheusClientKey, Valid: true}
			}

			result, err = session.ExecCtx(ctx,
				`INSERT INTO onec_cluster_app (
					cluster_uuid, app_name, app_code, app_type, is_default,
					app_url, port, protocol, auth_enabled, auth_type,
					username, password, token, access_key, access_secret,
					tls_enabled, insecure_skip_verify, ca_cert, client_cert, client_key,
					status, created_by, updated_by
				) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
				uuid,                            // cluster_uuid
				"Prometheus",                    // app_name
				"prometheus",                    // app_code
				1,                               // app_type: 1=monitoring
				1,                               // is_default: 设置为默认
				in.PrometheusUrl,                // app_url
				in.PrometheusPort,               // port
				in.PrometheusProtocol,           // protocol
				in.PrometheusAuthEnabled,        // auth_enabled
				in.PrometheusAuthType,           // auth_type
				in.PrometheusUsername,           // username
				in.PrometheusPassword,           // password
				prometheusToken,                 // token
				in.PrometheusAccessKey,          // access_key
				in.PrometheusAccessSecret,       // access_secret
				in.PrometheusTlsEnabled,         // tls_enabled
				in.PrometheusInsecureSkipVerify, // insecure_skip_verify
				prometheusCaCert,                // ca_cert
				prometheusClientCert,            // client_cert
				prometheusClientKey,             // client_key
				1,                               // status: 1=正常
				in.CreatedBy,                    // created_by
				in.CreatedBy,                    // updated_by
			)
			if err != nil {
				l.Errorf("插入 Prometheus 应用配置失败: %v", err)
				return errorx.Msg("添加 Prometheus 配置失败")
			}

			if rowsAffected, _ := result.RowsAffected(); rowsAffected == 0 {
				return errorx.Msg("添加 Prometheus 配置失败: 未插入任何记录")
			}
			l.Infof("Prometheus 应用配置插入成功")
		} else {
			l.Infof("步骤5.4: 跳过 Prometheus 配置（未启用）")
		}

		return nil
	})

	if err != nil {
		l.Errorf("添加集群失败（事务回滚）: %v", err)
		return nil, err
	}

	l.Infof("集群创建事务提交成功, 集群UUID: %s", uuid)

	// ==================== 6. 异步执行集群同步操作 ====================
	go func() {
		syncCtx := context.Background()

		if err := l.svcCtx.SyncOperator.SyncOneCLuster(syncCtx, uuid, in.CreatedBy, true); err != nil {
			l.Errorf("集群同步失败: %v", err)
		} else {
			l.Infof("集群同步成功")
		}
	}()

	return &pb.AddClusterResponse{
		Uuid: uuid,
	}, nil
}

// updateClusterStatus 更新集群状态
func (l *ClusterAddLogic) updateClusterStatus(ctx context.Context, uuid string, status int64) error {
	cluster, err := l.svcCtx.OnecClusterModel.FindOneByUuid(ctx, uuid)
	if err != nil {
		return err
	}

	cluster.Status = status
	return l.svcCtx.OnecClusterModel.Update(ctx, cluster)
}

// IsClusterConnectOk 判断集群连接是否正常
func (l *ClusterAddLogic) IsClusterConnectOk(in *pb.AddClusterRequest, clusterUuid string) error {
	l.Infof("开始测试集群连接，临时集群 UUID: %s", clusterUuid)

	clusterConfig := cluster.Config{
		Name:     clusterUuid,
		ID:       clusterUuid,
		AuthType: cluster.AuthType(in.AuthType),
	}

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

	l.Info("添加临时集群到管理器")
	if err := l.svcCtx.K8sManager.AddCluster(clusterConfig); err != nil {
		l.Errorf("添加临时集群失败: %v", err)
		return fmt.Errorf("添加临时集群失败: %w", err)
	}
	l.Info("临时集群添加成功")

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

	l.Info("开始健康检查")
	if err := client.HealthCheck(l.ctx); err != nil {
		l.Errorf("集群健康检查失败: %v", err)
		return fmt.Errorf("集群健康检查失败: %w", err)
	}

	l.Info("集群连接测试成功")
	return nil
}
