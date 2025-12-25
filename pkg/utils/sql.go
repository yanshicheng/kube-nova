package utils

import (
	"fmt"
	"strings"
)

func ConvertUint64SliceToInterfaceSlice(input []uint64) []interface{} {
	result := make([]interface{}, len(input))
	for i, v := range input {
		result[i] = v
	}
	return result
}

func BuildInCondition(field string, count int) string {
	placeholders := make([]string, count)
	for i := range placeholders {
		placeholders[i] = "?"
	}
	return fmt.Sprintf("%s IN (%s)", field, strings.Join(placeholders, ","))
}
