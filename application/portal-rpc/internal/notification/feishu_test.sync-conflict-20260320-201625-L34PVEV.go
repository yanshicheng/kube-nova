package notification

import (
	"encoding/json"
	"testing"
)

// TestFeiShuMessageFormat 测试飞书消息格式是否符合官方文档
func TestFeiShuMessageFormat(t *testing.T) {
	formatter := NewMessageFormatter("测试平台", "https://example.com")

	// 模拟告警数据
	alerts := []*AlertInstance{
		{
			ID:          1,
			Instance:    "192.168.1.100:9100",
			AlertName:   "CPU使用率过高",
			Severity:    "warning",
			Status:      "firing",
			ClusterName: "测试集群",
			ProjectName: "测试项目",
			Annotations: map[string]string{
				"summary":     "CPU使用率过高",
				"description": "CPU使用率超过90%",
				"value":       "95%",
			},
			Duration:    300,
			RepeatCount: 1,
		},
	}

	opts := &AlertOptions{
		PortalName:  "测试平台",
		PortalUrl:   "https://example.com",
		ProjectName: "测试项目",
		ClusterName: "测试集群",
		Severity:    "warning",
	}

	// 生成飞书富文本消息
	title, content := formatter.FormatRichTextForFeiShu(opts, alerts)

	// 验证标题
	if title == "" {
		t.Error("标题不能为空")
	}

	// 验证内容格式
	if len(content) == 0 {
		t.Error("内容不能为空")
	}

	// 序列化为JSON验证格式
	jsonData, err := json.MarshalIndent(content, "", "  ")
	if err != nil {
		t.Errorf("JSON序列化失败: %v", err)
	}

	t.Logf("飞书消息标题: %s", title)
	t.Logf("飞书消息内容:\n%s", string(jsonData))

	// 验证每一行的格式
	for i, line := range content {
		for j, element := range line {
			// 检查必需字段
			if _, ok := element["tag"]; !ok {
				t.Errorf("第 %d 行第 %d 个元素缺少 tag 字段", i, j)
			}

			// 检查 text 标签
			if element["tag"] == "text" {
				if _, ok := element["text"]; !ok {
					t.Errorf("第 %d 行第 %d 个元素是 text 标签但缺少 text 字段", i, j)
				}
			}

			// 检查 a 标签
			if element["tag"] == "a" {
				if _, ok := element["text"]; !ok {
					t.Errorf("第 %d 行第 %d 个元素是 a 标签但缺少 text 字段", i, j)
				}
				if _, ok := element["href"]; !ok {
					t.Errorf("第 %d 行第 %d 个元素是 a 标签但缺少 href 字段", i, j)
				}
			}

			// 检查是否有不支持的 style 字段
			if _, ok := element["style"]; ok {
				t.Errorf("第 %d 行第 %d 个元素包含不支持的 style 字段", i, j)
			}
		}
	}
}

// TestFeiShuNotificationFormat 测试飞书通知消息格式
func TestFeiShuNotificationFormat(t *testing.T) {
	formatter := NewMessageFormatter("测试平台", "https://example.com")

	opts := &NotificationOptions{
		PortalName: "测试平台",
		PortalUrl:  "https://example.com",
		Title:      "系统通知",
		Content:    "这是一条测试通知消息",
	}

	// 生成飞书通知消息
	title, content := formatter.FormatNotificationForFeiShu(opts)

	// 验证标题
	if title == "" {
		t.Error("标题不能为空")
	}

	// 验证内容格式
	if len(content) == 0 {
		t.Error("内容不能为空")
	}

	// 序列化为JSON验证格式
	jsonData, err := json.MarshalIndent(content, "", "  ")
	if err != nil {
		t.Errorf("JSON序列化失败: %v", err)
	}

	t.Logf("飞书通知标题: %s", title)
	t.Logf("飞书通知内容:\n%s", string(jsonData))

	// 验证格式
	for i, line := range content {
		for j, element := range line {
			if _, ok := element["tag"]; !ok {
				t.Errorf("第 %d 行第 %d 个元素缺少 tag 字段", i, j)
			}

			// 检查是否有不支持的 style 字段
			if _, ok := element["style"]; ok {
				t.Errorf("第 %d 行第 %d 个元素包含不支持的 style 字段", i, j)
			}
		}
	}
}
