package managerservicelogic

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"
	"github.com/yanshicheng/kube-nova/common/k8smanager/cluster"
	"github.com/zeromicro/go-zero/core/stores/sqlx"

	"github.com/zeromicro/go-zero/core/logx"
)

type ClusterUpdateAuthInfoLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewClusterUpdateAuthInfoLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ClusterUpdateAuthInfoLogic {
	return &ClusterUpdateAuthInfoLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ClusterUpdateAuthInfo 修改集群认证信息
func (l *ClusterUpdateAuthInfoLogic) ClusterUpdateAuthInfo(in *pb.ClusterUpdateAuthInfoReq) (*pb.ClusterUpdateAuthInfoResp, error) {
	oldCluster, err := l.svcCtx.OnecClusterModel.FindOne(l.ctx, in.Id)
	if err != nil {
		l.Errorf("查询集群数据失败: %v", err)
		return nil, err
	}

	// 验证认证类型
	authtypeList := []string{"incluster", "certificate", "token", "kubeconfig"}
	if !IsInList(in.AuthType, authtypeList) {
		l.Errorf("不支持的认证类型: %s", in.AuthType)
		return nil, errorx.Msg("不支持的认证类型!")
	}

	auth, err := l.svcCtx.OnecClusterAuthModel.FindOneByClusterUuid(l.ctx, oldCluster.Uuid)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			l.Errorf("集群认证信息不存在 [clusterUuid=%s]", oldCluster.Uuid)
		}
		l.Errorf("查询集群认证信息失败: %v", err)
		return nil, errorx.Msg("查询集群认证信息失败")
	}

	// 开启事务
	err = l.svcCtx.OnecClusterModel.TransCtx(l.ctx, func(ctx context.Context, session sqlx.Session) error {
		// 判断是否变更了认证信息
		if in.AuthType != auth.AuthType ||
			in.ApiServerHost != auth.ApiServerHost ||
			in.Token != auth.Token ||
			in.CaFile != auth.CaFile ||
			in.CertFile != auth.CertFile ||
			in.KeyFile != auth.KeyFile ||
			in.InsecureSkipVerify != auth.InsecureSkipVerify ||
			(in.KubeFile != "" && in.KubeFile != getNullString(auth.Kubefile)) ||
			(in.CaCert != "" && in.CaCert != getNullString(auth.CaCert)) ||
			(in.ClientCert != "" && in.ClientCert != getNullString(auth.ClientCert)) ||
			(in.ClientKey != "" && in.ClientKey != getNullString(auth.ClientKey)) {

			l.Info("检测到认证信息变更，开始测试新配置")

			// 先测试连接是否正常
			if err := l.ClusterUpdateAuthInfoAndTest(in, oldCluster.Uuid); err != nil {
				l.Errorf("集群连接测试失败: %v", err)
				return errorx.Msg("集群连接测试失败，请检查认证配置")
			}

			l.Info("新认证配置测试成功，开始更新数据库")

			// 更新认证信息
			// string 类型字段直接赋值
			auth.AuthType = in.AuthType
			auth.ApiServerHost = in.ApiServerHost
			auth.Token = in.Token       // string 类型，直接赋值
			auth.CaFile = in.CaFile     // string 类型，直接赋值
			auth.CertFile = in.CertFile // string 类型，直接赋值
			auth.KeyFile = in.KeyFile   // string 类型，直接赋值
			auth.InsecureSkipVerify = in.InsecureSkipVerify

			// sql.NullString 类型字段需要包装
			if in.KubeFile != "" {
				auth.Kubefile = sql.NullString{String: in.KubeFile, Valid: true}
			} else {
				auth.Kubefile = sql.NullString{String: "", Valid: false}
			}

			if in.CaCert != "" {
				auth.CaCert = sql.NullString{String: in.CaCert, Valid: true}
			} else {
				auth.CaCert = sql.NullString{String: "", Valid: false}
			}

			if in.ClientCert != "" {
				auth.ClientCert = sql.NullString{String: in.ClientCert, Valid: true}
			} else {
				auth.ClientCert = sql.NullString{String: "", Valid: false}
			}

			if in.ClientKey != "" {
				auth.ClientKey = sql.NullString{String: in.ClientKey, Valid: true}
			} else {
				auth.ClientKey = sql.NullString{String: "", Valid: false}
			}

			// 在事务中更新认证信息
			if err := l.svcCtx.OnecClusterAuthModel.Update(ctx, auth); err != nil {
				l.Errorf("更新集群认证信息失败: %v", err)
				return errorx.Msg("更新集群认证信息失败")
			}

			l.Info("数据库认证信息更新成功")
		} else {
			l.Info("认证信息未发生变更，跳过更新")
		}

		// 事务成功，返回nil
		return nil
	})

	// 检查事务执行结果
	if err != nil {
		return nil, err
	}

	if err := l.svcCtx.OnecClusterAuthModel.DeleteCacheByClusterUuid(l.ctx, oldCluster.Uuid); err != nil {
		l.Errorf("删除集群认证信息缓存失败 [clusterUuid=%s]: %v", oldCluster.Uuid, err)
	} else {
		l.Infof("成功删除集群认证信息缓存 [clusterUuid=%s]", oldCluster.Uuid)
	}

	l.Info("开始更新管理器中的集群配置")
	if err := l.updateManagerCluster(oldCluster.Uuid, in); err != nil {
		l.Errorf("更新管理器中的集群配置失败: %v", err)
	} else {
		l.Info("管理器中的集群配置更新成功")
	}

	// 返回成功响应
	return &pb.ClusterUpdateAuthInfoResp{}, nil
}

func (l *ClusterUpdateAuthInfoLogic) ClusterUpdateAuthInfoAndTest(in *pb.ClusterUpdateAuthInfoReq, clusterUuid string) error {
	l.Infof("开始测试新的认证配置，集群 UUID: %s", clusterUuid)

	// 生成临时测试 UUID（避免影响现有集群）
	testUuid := clusterUuid + "-test"

	// 构建测试集群配置
	clusterConfig := cluster.Config{
		Name:     testUuid,
		ID:       testUuid,
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

	// 添加临时测试集群
	l.Info("添加临时测试集群到管理器")
	if err := l.svcCtx.K8sManager.AddCluster(clusterConfig); err != nil {
		l.Errorf("添加临时测试集群失败: %v", err)
		return fmt.Errorf("添加临时测试集群失败: %w", err)
	}
	l.Info("临时测试集群添加成功")

	// 使用 defer 确保清理临时测试集群
	defer func() {
		l.Infof("删除临时测试集群: %s", testUuid)
		if err := l.svcCtx.K8sManager.RemoveCluster(testUuid); err != nil {
			l.Errorf("删除临时测试集群失败: %v", err)
		} else {
			l.Info("临时测试集群删除成功")
		}
	}()

	// 获取临时测试集群客户端
	client, err := l.svcCtx.K8sManager.GetCluster(l.ctx, testUuid)
	if err != nil {
		l.Errorf("获取临时测试集群客户端失败: %v", err)
		return fmt.Errorf("获取临时测试集群客户端失败: %w", err)
	}

	// 执行健康检查
	l.Info("开始健康检查")
	if err := client.HealthCheck(l.ctx); err != nil {
		l.Errorf("集群健康检查失败: %v", err)
		return fmt.Errorf("集群健康检查失败: %w", err)
	}

	l.Info("新认证配置测试成功")
	return nil
}

// updateManagerCluster 更新管理器中的集群配置
func (l *ClusterUpdateAuthInfoLogic) updateManagerCluster(clusterUuid string, in *pb.ClusterUpdateAuthInfoReq) error {
	// 构建新的集群配置
	clusterConfig := cluster.Config{
		Name:     clusterUuid,
		ID:       clusterUuid,
		AuthType: cluster.AuthType(in.AuthType),
	}

	// 根据认证类型设置配置
	switch in.AuthType {
	case "kubeconfig":
		clusterConfig.KubeConfigData = in.KubeFile
	case "token":
		clusterConfig.APIServer = in.ApiServerHost
		clusterConfig.Token = in.Token
		clusterConfig.Insecure = in.InsecureSkipVerify == 1
	case "certificate":
		clusterConfig.APIServer = in.ApiServerHost
		clusterConfig.CertFile = in.CertFile
		clusterConfig.KeyFile = in.KeyFile
		clusterConfig.CAFile = in.CaFile
		clusterConfig.Insecure = in.InsecureSkipVerify == 1
	case "incluster":
		// incluster 模式
	}

	// 更新管理器中的集群配置
	if err := l.svcCtx.K8sManager.UpdateCluster(clusterConfig); err != nil {
		l.Errorf("更新管理器中的集群配置失败: %v", err)
		return err
	}

	return nil
}
