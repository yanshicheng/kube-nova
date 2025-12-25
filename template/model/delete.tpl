func (m *default{{.upperStartCamelObject}}Model) Delete(ctx context.Context, {{.lowerStartCamelPrimaryKey}} {{.dataType}}) error {
	{{if .withCache}}{{if .containsIndexCache}}data, err:=m.FindOne(ctx, {{.lowerStartCamelPrimaryKey}})
	if err!=nil{
		return err
	}

{{end}}	{{.keys}}
    _, err {{if .containsIndexCache}}={{else}}:={{end}} m.ExecCtx(ctx, func(ctx context.Context, conn sqlx.SqlConn) (result sql.Result, err error) {
		query := fmt.Sprintf("delete from %s where {{.originalPrimaryKey}} = {{if .postgreSql}}$1{{else}}?{{end}}", m.table)
		return conn.ExecCtx(ctx, query, {{.lowerStartCamelPrimaryKey}})
	}, {{.keyValues}}){{else}}query := fmt.Sprintf("delete from %s where {{.originalPrimaryKey}} = {{if .postgreSql}}$1{{else}}?{{end}}", m.table)
		_,err:=m.conn.ExecCtx(ctx, query, {{.lowerStartCamelPrimaryKey}}){{end}}
	return err
}



func (m *default{{.upperStartCamelObject}}Model) DeleteSoft(ctx context.Context, {{.lowerStartCamelPrimaryKey}} {{.dataType}}) error {
	{{if .withCache}}{{if .containsIndexCache}}data, err:=m.FindOne(ctx, {{.lowerStartCamelPrimaryKey}})
	if err!=nil{
		return err
	}
	// 如果记录已软删除，无需再次删除
	if data.IsDeleted == 1 {
		return nil
	}
{{end}}	{{.keys}}
    _, err {{if .containsIndexCache}}={{else}}:={{end}} m.ExecCtx(ctx, func(ctx context.Context, conn sqlx.SqlConn) (result sql.Result, err error) {
		query := fmt.Sprintf("update %s set {{if .postgreSql}}is_deleted{{else}}`is_deleted`{{end}} = 1 where {{.originalPrimaryKey}} = {{if .postgreSql}}$1{{else}}?{{end}}", m.table)
		return conn.ExecCtx(ctx, query, {{.lowerStartCamelPrimaryKey}})
	}, {{.keyValues}}){{else}}query := fmt.Sprintf("update %s set {{if .postgreSql}}is_deleted{{else}}`is_deleted`{{end}} = 1 where {{.originalPrimaryKey}} = {{if .postgreSql}}$1{{else}}?{{end}}", m.table)
		_,err:=m.conn.ExecCtx(ctx, query, {{.lowerStartCamelPrimaryKey}}){{end}}
	return err
}




func (m *default{{.upperStartCamelObject}}Model) TransCtx(ctx context.Context, fn func(context.Context, sqlx.Session) error) error {
	return m.TransactCtx(ctx, func(ctx context.Context, session sqlx.Session) error {
		return fn(ctx, session)
	})
}

func (m *default{{.upperStartCamelObject}}Model) TransOnSql(ctx context.Context, session sqlx.Session,  {{.lowerStartCamelPrimaryKey}} {{.dataType}}, sqlStr string, args ...any) (sql.Result, error) {
	// 校验 SQL 字符串是否包含表名占位符
	// 如果没有 {table} 占位符，直接使用原始 SQL（兼容直接写表名的情况）
	var query string
	if strings.Contains(sqlStr, "{table}") {
		query = strings.ReplaceAll(sqlStr, "{table}", m.table)
	} else {
		query = sqlStr
	}

	// 如果 id 不为零值并且需要处理缓存逻辑
	if !isZeroValue({{.lowerStartCamelPrimaryKey}}) {
		{{if .withCache}}
		// 查询数据以获取缓存键（如果需要）
		{{if .containsIndexCache}}data, err := m.FindOne(ctx, {{.lowerStartCamelPrimaryKey}})
		if err != nil {
			return nil, err
		}
		{{end}}

		// 处理缓存逻辑
		{{.keys}}
		// 执行带缓存处理的 SQL 操作
		return m.ExecCtx(ctx, func(ctx context.Context, conn sqlx.SqlConn) (sql.Result, error) {
			return session.ExecCtx(ctx, query, args...)
		}, {{.keyValues}})
		{{else}}
		// 不使用缓存模式，直接通过 session 执行 SQL
		return session.ExecCtx(ctx, query, args...)
		{{end}}
	}

	// 如果 id 为零值，直接通过 session 执行 SQL（无需处理缓存）
	return session.ExecCtx(ctx, query, args...)
}

func (m *default{{.upperStartCamelObject}}Model) ExecSql(ctx context.Context, {{.lowerStartCamelPrimaryKey}} {{.dataType}}, sqlStr string, args ...any) (sql.Result, error) {
	// 校验 SQL 字符串是否包含表名占位符
	// 如果没有 {table} 占位符，直接使用原始 SQL（兼容直接写表名的情况）
	var query string
	if strings.Contains(sqlStr, "{table}") {
		query = strings.ReplaceAll(sqlStr, "{table}", m.table)
	} else {
		query = sqlStr
	}

	// 如果 id 不为零值并且需要处理缓存逻辑
	if !isZeroValue({{.lowerStartCamelPrimaryKey}}) {
		{{if .withCache}}
		// 查询数据以获取缓存键（如果需要）
		{{if .containsIndexCache}}data, err := m.FindOne(ctx, {{.lowerStartCamelPrimaryKey}})
		if err != nil {
			return nil, err
		}
		{{end}}

		// 处理缓存逻辑
		{{.keys}}
		// 执行带缓存处理的 SQL 操作
		return m.ExecCtx(ctx, func(ctx context.Context, conn sqlx.SqlConn) (sql.Result, error) {
			return conn.ExecCtx(ctx, query, args...)
		}, {{.keyValues}})
		{{else}}
		// 不使用缓存模式，直接执行 SQL
		return m.conn.ExecCtx(ctx, query, args...)
		{{end}}
	}

	// 如果 id 为零值，直接执行 SQL（无需处理缓存）
	{{if .withCache}}
	return m.ExecNoCacheCtx(ctx, query, args...)
	{{else}}
	return m.conn.ExecCtx(ctx, query, args...)
	{{end}}
}
