package alert

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"time"

	"github.com/yanshicheng/kube-nova/application/manager-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/zeromicro/go-zero/core/logx"
)

type ExportAllAlertRulesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewExportAllAlertRulesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ExportAllAlertRulesLogic {
	return &ExportAllAlertRulesLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// ExportAllAlertRules 导出全部告警规则为 ZIP 压缩包
func (l *ExportAllAlertRulesLogic) ExportAllAlertRules() (zipData []byte, fileName string, err error) {
	// 调用 RPC 服务导出全部告警规则
	result, err := l.svcCtx.ManagerRpc.AlertRulesExportAll(l.ctx, &pb.ExportAllAlertRulesReq{})
	if err != nil {
		l.Errorf("调用 RPC 导出全部告警规则失败: %v", err)
		return nil, "", fmt.Errorf("导出告警规则失败: %v", err)
	}

	if len(result.Files) == 0 {
		return nil, "", fmt.Errorf("没有可导出的告警规则")
	}

	// 创建 ZIP 文件
	buf := new(bytes.Buffer)
	zipWriter := zip.NewWriter(buf)

	// 将每个 YAML 文件写入 ZIP
	for _, file := range result.Files {
		// 创建 ZIP 文件条目
		fileWriter, err := zipWriter.Create(file.FileName)
		if err != nil {
			l.Errorf("创建 ZIP 文件条目失败: %s, error: %v", file.FileName, err)
			continue
		}

		// 写入 YAML 内容
		if _, err := fileWriter.Write([]byte(file.YamlStr)); err != nil {
			l.Errorf("写入 YAML 内容到 ZIP 失败: %s, error: %v", file.FileName, err)
			continue
		}

		l.Infof("成功添加文件到 ZIP: %s", file.FileName)
	}

	// 关闭 ZIP writer
	if err := zipWriter.Close(); err != nil {
		l.Errorf("关闭 ZIP writer 失败: %v", err)
		return nil, "", fmt.Errorf("生成 ZIP 文件失败")
	}

	// 生成文件名（带时间戳）
	zipFileName := fmt.Sprintf("alert-rules-%s.zip", time.Now().Format("20060102-150405"))

	l.Infof("成功生成告警规则 ZIP 文件: %s, 包含 %d 个文件", zipFileName, len(result.Files))

	return buf.Bytes(), zipFileName, nil
}
