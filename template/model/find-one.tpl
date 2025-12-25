func (m *default{{.upperStartCamelObject}}Model) FindOne(ctx context.Context, {{.lowerStartCamelPrimaryKey}} {{.dataType}}) (*{{.upperStartCamelObject}}, error) {
	{{if .withCache}}{{.cacheKey}}
	var resp {{.upperStartCamelObject}}
	err := m.QueryRowCtx(ctx, &resp, {{.cacheKeyVariable}}, func(ctx context.Context, conn sqlx.SqlConn, v any) error {
		query :=  fmt.Sprintf("select %s from %s where {{.originalPrimaryKey}} = {{if .postgreSql}}$1{{else}}?{{end}} limit 1", {{.lowerStartCamelObject}}Rows, m.table)
		return conn.QueryRowCtx(ctx, v, query, {{.lowerStartCamelPrimaryKey}})
	})
	switch err {
	case nil:
		return &resp, nil
	case sqlc.ErrNotFound:
		return nil, ErrNotFound
	default:
		return nil, err
	}{{else}}query := fmt.Sprintf("select %s from %s where {{.originalPrimaryKey}} = {{if .postgreSql}}$1{{else}}?{{end}} limit 1", {{.lowerStartCamelObject}}Rows, m.table)
	var resp {{.upperStartCamelObject}}
	err := m.conn.QueryRowCtx(ctx, &resp, query, {{.lowerStartCamelPrimaryKey}})
	switch err {
	case nil:
		return &resp, nil
	case sqlx.ErrNotFound:
		return nil, ErrNotFound
	default:
		return nil, err
	}{{end}}
}


func (m *default{{.upperStartCamelObject}}Model) Search(ctx context.Context,  orderStr string, isAsc bool, page, pageSize uint64, queryStr string, args ...any) ([]*{{.upperStartCamelObject}}, uint64, error) {
	// 确保分页参数有效
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}

	// 构造查询条件
	// 添加 is_deleted = 0 条件，保证只查询未软删除数据
	// 初始化 WHERE 子句，根据数据库类型选择正确的引用符号
	{{if .postgreSql}}
	where := "WHERE is_deleted = 0"
	if queryStr != "" {
		where = fmt.Sprintf("WHERE %s AND is_deleted = 0", queryStr)
	}
	{{else}}
	where := "WHERE `is_deleted` = 0"
	if queryStr != "" {
		where = fmt.Sprintf("WHERE %s AND `is_deleted` = 0", queryStr)
	}
	{{end}}

	// 根据 isAsc 参数确定排序方式
	sortDirection := "ASC"
	if !isAsc {
		sortDirection = "DESC"
	}

	// 安全处理排序字段，防止 SQL 注入
	// 只允许字母、数字、下划线，并移除危险字符
	if orderStr == "" {
		orderStr = fmt.Sprintf("ORDER BY id %s", sortDirection)
	} else {
		orderStr = strings.TrimSpace(orderStr)
		// 移除可能的 SQL 注入字符：分号、注释符号、引号等
		orderStr = strings.NewReplacer(
			";", "",
			"--", "",
			"/*", "",
			"*/", "",
			"'", "",
			"\"", "",
			"\\", "",
		).Replace(orderStr)

		if !strings.HasPrefix(strings.ToUpper(orderStr), "ORDER BY") {
			orderStr = "ORDER BY " + orderStr
		}
		orderStr = fmt.Sprintf("%s %s", orderStr, sortDirection)
	}

	countQuery := fmt.Sprintf("SELECT COUNT(1) FROM %s %s", m.table, where)

	var total uint64
	var resp []*{{.upperStartCamelObject}}
	err := m.QueryRowNoCacheCtx(ctx, &total, countQuery, args...)
	if err != nil {
		return nil, 0, err
	}
	// 搜索无结果是正常情况，返回空切片而非错误
	// 这样上层调用方可以直接判断切片长度，无需额外处理错误
	if total == 0 {
		return []*{{.upperStartCamelObject}}{}, 0, nil
	}
	offset := (page - 1) * pageSize
	{{if .postgreSql}}
	dataQuery := fmt.Sprintf("SELECT %s FROM %s %s %s LIMIT %d OFFSET %d", {{.lowerStartCamelObject}}Rows, m.table, where, orderStr, pageSize, offset)
	{{else}}
	dataQuery := fmt.Sprintf("SELECT %s FROM %s %s %s LIMIT %d,%d", {{.lowerStartCamelObject}}Rows, m.table, where, orderStr, offset, pageSize)
	{{end}}

	err = m.QueryRowsNoCacheCtx(ctx, &resp, dataQuery, args...)
	if err != nil {
		return nil, 0, err
	}

	return resp, total, nil
}

func (m *default{{.upperStartCamelObject}}Model) SearchNoPage(ctx context.Context, orderStr string, isAsc bool,  queryStr string, args ...any) ([]*{{.upperStartCamelObject}}, error) {
	// 处理 nil 参数，确保后续操作安全
	if args == nil {
		args = []any{}
	}

	// 初始化 WHERE 子句，根据数据库类型选择正确的引用符号
	{{if .postgreSql}}
	where := "WHERE is_deleted = 0"
	{{else}}
	where := "WHERE `is_deleted` = 0"
	{{end}}

	// 处理查询条件和参数
	var finalArgs []any
	if queryStr != "" {
		{{if .postgreSql}}
		where = fmt.Sprintf("WHERE %s AND is_deleted = 0", queryStr)
		{{else}}
		where = fmt.Sprintf("WHERE %s AND `is_deleted` = 0", queryStr)
		{{end}}
		// 只有当有查询条件时才使用参数
		if len(args) > 0 {
			finalArgs = args
		}
	}

	// 根据 isAsc 参数确定排序方式
	sortDirection := "ASC"
	if !isAsc {
		sortDirection = "DESC"
	}

	// 安全处理排序字段，防止 SQL 注入
	if orderStr == "" {
		orderStr = fmt.Sprintf("ORDER BY id %s", sortDirection)
	} else {
		orderStr = strings.TrimSpace(orderStr)
		// 移除可能的 SQL 注入字符
		orderStr = strings.NewReplacer(
			";", "",
			"--", "",
			"/*", "",
			"*/", "",
			"'", "",
			"\"", "",
			"\\", "",
		).Replace(orderStr)

		if !strings.HasPrefix(strings.ToUpper(orderStr), "ORDER BY") {
			orderStr = "ORDER BY " + orderStr
		}
		orderStr = fmt.Sprintf("%s %s", orderStr, sortDirection)
	}

	dataQuery := fmt.Sprintf("SELECT %s FROM %s %s %s", {{.lowerStartCamelObject}}Rows, m.table, where, orderStr)
	var resp []*{{.upperStartCamelObject}}

	// 根据是否有参数来调用，避免传递空参数导致问题
	var err error
	if len(finalArgs) > 0 {
		err = m.QueryRowsNoCacheCtx(ctx, &resp, dataQuery, finalArgs...)
	} else {
		err = m.QueryRowsNoCacheCtx(ctx, &resp, dataQuery)
	}

	switch err {
	case nil:
		// 返回结果，如果无数据则返回空切片
		if resp == nil {
			return []*{{.upperStartCamelObject}}{}, nil
		}
		return resp, nil
	case sqlx.ErrNotFound:
		// 无数据时返回空切片而非错误，保持与 Search 方法一致的行为
		return []*{{.upperStartCamelObject}}{}, nil
	default:
		return nil, err
	}
}
