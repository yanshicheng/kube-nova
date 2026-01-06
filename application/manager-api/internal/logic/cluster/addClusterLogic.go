package cluster

import (
	"context"
	"net/http"

	"github.com/yanshicheng/kube-nova/application/manager-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/zeromicro/go-zero/core/logx"
)

type AddClusterLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAddClusterLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AddClusterLogic {
	return &AddClusterLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AddClusterLogic) AddCluster(r *http.Request, req *types.AddClusterRequest) (resp string, err error) {
	// 获取当前用户信息
	username, ok := l.ctx.Value("username").(string)
	if !ok || username == "" {
		username = "system"
	}

	_ = r.ParseMultipartForm(32 << 20)

	// 默认头像地址 (建议替换为真实的默认图片 URL)
	avatarUrl := "/public/kube-nova.png"
	//var fileData []byte
	//var fileName string

	//file, handler, errFile := r.FormFile("avatar")
	//if errFile == nil {
	//	defer func(file multipart.File) {
	//		if err := file.Close(); err != nil {
	//			l.Errorf("关闭文件失败: %v", err)
	//		}
	//	}(file)
	//
	//	data, err := io.ReadAll(file)
	//	if err != nil {
	//		l.Errorf("读取头像文件失败: %v", err)
	//	} else {
	//		fileData = data
	//		fileName = handler.Filename
	//		l.Infof("检测到文件流上传: %s (大小: %d)", fileName, len(fileData))
	//	}
	//} else {
	//	// 你的截图显示前端传的是 Base64 字符串，这里进行兼容
	//	base64Str := r.PostFormValue("avatar")
	//	if base64Str != "" {
	//		// 检查是否包含 data URI scheme 前缀 (如 data:image/png;base64,)
	//		if idx := strings.Index(base64Str, ","); idx > -1 {
	//			base64Str = base64Str[idx+1:] // 去掉前缀
	//		}
	//
	//		// 解码 Base64
	//		decoded, errDecode := base64.StdEncoding.DecodeString(base64Str)
	//		if errDecode != nil {
	//			l.Errorf("Base64 头像解码失败: %v", errDecode)
	//		} else {
	//			fileData = decoded
	//			// Base64 没有文件名，生成一个带时间戳的文件名
	//			fileName = fmt.Sprintf("avatar_%d.png", time.Now().Unix())
	//			l.Infof("检测到 Base64 字符串上传 (解码后大小: %d)", len(fileData))
	//		}
	//	}
	//}
	//
	//// 如果获取到了图片数据，执行 RPC 上传
	//if len(fileData) > 0 {
	//	image, err := l.svcCtx.StoreRpc.UploadImage(l.ctx, &storageservice.UploadImageRequest{
	//		ImageData: fileData,
	//		FileName:  fileName,
	//		Project:   "cluster",
	//	})
	//	if err != nil {
	//		l.Errorf("调用存储服务上传头像失败: %v", err)
	//		return "", fmt.Errorf("头像上传失败: %w", err)
	//	}
	//	avatarUrl = image.ImageUri
	//	l.Infof("头像上传成功: %s", avatarUrl)
	//} else {
	//	l.Infof("未检测到有效头像数据，将使用默认头像")
	//}

	// ==================== 头像处理逻辑结束 ====================

	// 调用RPC创建集群
	rpcReq := &pb.AddClusterRequest{
		// 基本信息
		Name:          req.Name,
		Avatar:        avatarUrl,
		Description:   req.Description,
		ClusterType:   req.ClusterType,
		Environment:   req.Environment,
		Region:        req.Region,
		Zone:          req.Zone,
		Datacenter:    req.Datacenter,
		Provider:      req.Provider,
		IsManaged:     req.IsManaged,
		NodeLb:        req.NodeLb,
		MasterLb:      req.MasterLb,
		IngressDomain: req.IngressDomain,
		// 认证信息
		AuthType:           req.AuthType,
		ApiServerHost:      req.ApiServerHost,
		KubeFile:           req.KubeFile,
		Token:              req.Token,
		CaCert:             req.CaCert,
		CaFile:             req.CaFile,
		ClientCert:         req.ClientCert,
		CertFile:           req.CertFile,
		ClientKey:          req.ClientKey,
		KeyFile:            req.KeyFile,
		InsecureSkipVerify: req.InsecureSkipVerify,
		CreatedBy:          username,
		UpdatedBy:          username,
		// 费用配置
		PriceConfigId:    req.PriceConfigId,
		BillingStartTime: req.BillingStartTime,
		// Prometheus 配置
		EnablePrometheus:             req.EnablePrometheus == 1,
		PrometheusUrl:                req.PrometheusUrl,
		PrometheusPort:               req.PrometheusPort,
		PrometheusProtocol:           req.PrometheusProtocol,
		PrometheusAuthEnabled:        req.PrometheusAuthEnabled,
		PrometheusAuthType:           req.PrometheusAuthType,
		PrometheusUsername:           req.PrometheusUsername,
		PrometheusPassword:           req.PrometheusPassword,
		PrometheusToken:              req.PrometheusToken,
		PrometheusAccessKey:          req.PrometheusAccessKey,
		PrometheusAccessSecret:       req.PrometheusAccessSecret,
		PrometheusTlsEnabled:         req.PrometheusTlsEnabled,
		PrometheusInsecureSkipVerify: req.PrometheusInsecureSkipVerify,
		PrometheusCaCert:             req.PrometheusCaCert,
		PrometheusClientCert:         req.PrometheusClientCert,
		PrometheusClientKey:          req.PrometheusClientKey,
	}
	_, err = l.svcCtx.ManagerRpc.ClusterAdd(l.ctx, rpcReq)
	if err != nil {
		l.Errorf("RPC调用创建集群失败: %v", err)
		return "", err
	}

	return "集群创建成功", nil
}
