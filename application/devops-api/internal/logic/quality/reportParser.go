package quality

import "github.com/yanshicheng/kube-nova/common/devops/qualityreport"

func parseScanReport(tool, parser, reportFormat string, data []byte) (string, string, error) {
	return qualityreport.ParseScanReport(tool, parser, reportFormat, data)
}
