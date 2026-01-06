package billing

import (
	"fmt"
	"math"
	"time"
)

// Calculator 费用计算器
type Calculator struct{}

// NewCalculator 创建计算器实例
func NewCalculator() *Calculator {
	return &Calculator{}
}

// CalculateBillingPeriod 计算计费周期
// startTime: 计费开始时间
// endTime: 计费结束时间
// 返回计费时长（小时），保留4位小数
func (c *Calculator) CalculateBillingPeriod(startTime, endTime time.Time) *BillingPeriod {
	duration := endTime.Sub(startTime)
	hours := duration.Hours()
	// 保留4位小数，避免精度问题
	hours = math.Round(hours*10000) / 10000
	// 确保时长不为负数
	if hours < 0 {
		hours = 0
	}
	return &BillingPeriod{
		StartTime:    startTime,
		EndTime:      endTime,
		BillingHours: hours,
	}
}

// CalculateCost 计算各项费用
// info: 项目集群资源信息
// price: 价格配置
// billingHours: 计费时长（小时）
// 返回账单明细
func (c *Calculator) CalculateCost(info *ProjectClusterInfo, price *PriceConfig, billingHours float64) *StatementDetail {
	detail := &StatementDetail{
		BillingHours: billingHours,
		// 单价快照
		PriceCpu:           price.CpuPrice,
		PriceMemory:        price.MemoryPrice,
		PriceStorage:       price.StoragePrice,
		PriceGpu:           price.GpuPrice,
		PricePod:           price.PodPrice,
		PriceManagementFee: price.ManagementFee,
	}

	// 计算CPU费用：容量 * 单价 * 时长
	detail.CpuCost = c.roundCost(info.CpuCapacity * price.CpuPrice * billingHours)

	// 计算内存费用：容量 * 单价 * 时长
	detail.MemoryCost = c.roundCost(info.MemCapacity * price.MemoryPrice * billingHours)

	// 计算存储费用：配额 * 单价 * 时长
	detail.StorageCost = c.roundCost(info.StorageLimit * price.StoragePrice * billingHours)

	// 计算GPU费用：容量 * 单价 * 时长
	detail.GpuCost = c.roundCost(info.GpuCapacity * price.GpuPrice * billingHours)

	// 计算Pod费用：配额 * 单价 * 时长
	detail.PodCost = c.roundCost(float64(info.PodsLimit) * price.PodPrice * billingHours)

	// 计算管理费：管理费单价 * 时长
	detail.ManagementFee = c.roundCost(price.ManagementFee * billingHours)

	// 计算资源费用合计
	detail.ResourceCostTotal = c.roundCost(
		detail.CpuCost +
			detail.MemoryCost +
			detail.StorageCost +
			detail.GpuCost +
			detail.PodCost,
	)

	// 计算总费用
	detail.TotalAmount = c.roundCost(detail.ResourceCostTotal + detail.ManagementFee)

	return detail
}

// roundCost 费用保留2位小数
func (c *Calculator) roundCost(cost float64) float64 {
	return math.Round(cost*100) / 100
}

// FormatCapacity 格式化容量值为字符串（用于存储到账单快照字段）
func (c *Calculator) FormatCapacity(value float64, unit string) string {
	if unit == "" {
		return fmt.Sprintf("%.2f", value)
	}
	return fmt.Sprintf("%.2f%s", value, unit)
}

// FormatCpuCapacity 格式化CPU容量
func (c *Calculator) FormatCpuCapacity(cores float64) string {
	return c.FormatCapacity(cores, "")
}

// FormatMemoryCapacity 格式化内存容量
func (c *Calculator) FormatMemoryCapacity(gib float64) string {
	return c.FormatCapacity(gib, "Gi")
}

// FormatStorageCapacity 格式化存储容量
func (c *Calculator) FormatStorageCapacity(gib float64) string {
	return c.FormatCapacity(gib, "Gi")
}

// FormatGpuCapacity 格式化GPU容量
func (c *Calculator) FormatGpuCapacity(count float64) string {
	return c.FormatCapacity(count, "")
}
