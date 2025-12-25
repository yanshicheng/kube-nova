package utils

import (
	"fmt"

	"k8s.io/apimachinery/pkg/api/resource"
)

// 1. 将内存字符串转换为 GiB（返回 float64 更精确，如果需要 int64 可以向下取整）
// 例如: "2Gi" -> 2, "2048Mi" -> 2, "1G" -> 0 (因为 1G = 0.931 GiB)
func MemoryToGiB(memoryStr string) (float64, error) {
	if memoryStr == "" || memoryStr == "0" {
		return 0, nil
	}

	quantity, err := resource.ParseQuantity(memoryStr)
	if err != nil {
		return 0, fmt.Errorf("failed to parse memory quantity: %v", err)
	}

	// 转换为字节
	bytes := quantity.Value()

	// 1 GiB = 1024 * 1024 * 1024 bytes = 1073741824 bytes
	gib := float64(bytes) / 1073741824.0

	return gib, nil
}

// 2. 将内存字符串转换为字节
// 例如: "1Gi" -> 1073741824, "1G" -> 1000000000, "512Mi" -> 536870912
func MemoryToBytes(memoryStr string) (int64, error) {
	if memoryStr == "" || memoryStr == "0" {
		return 0, nil
	}

	quantity, err := resource.ParseQuantity(memoryStr)
	if err != nil {
		return 0, fmt.Errorf("failed to parse memory quantity: %v", err)
	}

	// 直接返回字节数
	return quantity.Value(), nil
}

// 3. 将 CPU 字符串转换为核心数
// 例如: "1" -> 1.0, "0.5" -> 0.5, "1000m" -> 1.0, "500m" -> 0.5
func CPUToCores(cpuStr string) (float64, error) {
	if cpuStr == "" || cpuStr == "0" {
		return 0, nil
	}

	quantity, err := resource.ParseQuantity(cpuStr)
	if err != nil {
		return 0, fmt.Errorf("failed to parse CPU quantity: %v", err)
	}

	// 获取毫核数（millicores）
	milliCores := quantity.MilliValue()

	// 转换为核心数：1000 millicores = 1 core
	cores := float64(milliCores) / 1000.0

	return cores, nil
}

func GPUToCount(gpuStr string) (float64, error) {
	if gpuStr == "" || gpuStr == "0" {
		return 0, nil
	}

	quantity, err := resource.ParseQuantity(gpuStr)
	if err != nil {
		return 0, fmt.Errorf("failed to parse GPU quantity: %v", err)
	}

	// GPU 通常以整数或小数表示，使用 AsApproximateFloat64 获取浮点数
	gpuCount := quantity.AsApproximateFloat64()

	return gpuCount, nil
}
