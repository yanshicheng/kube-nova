package vars

const (
	Page       uint64 = 1
	PageSize   uint64 = 10
	OrderField string = "id"
	// 排序规则
	OrderType string = "desc" // 降序=desc，升序=asc
)

// jwtKey
const (
	AccessSecret  = "sxxxxxxassasasasqwqdwqfqewfohiwhdioqwbd"
	AccessExpire  = 86400
	RefreshSecret = "sxxxxxxassasasasqwqdwqfqewfohiwhdioqwbd"
	RefreshExpire = 86400 * 7
)

// 项目版本信息
const (
	ProjectName = "Onec"
	ProjectVer  = "v0.0.1"
)
