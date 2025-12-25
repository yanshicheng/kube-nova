Delete(ctx context.Context, {{.lowerStartCamelPrimaryKey}} {{.dataType}}) error
DeleteSoft(ctx context.Context, {{.lowerStartCamelPrimaryKey}} {{.dataType}}) error
TransCtx(ctx context.Context, fn func(context.Context, sqlx.Session) error) error
TransOnSql(ctx context.Context, session sqlx.Session, {{.lowerStartCamelPrimaryKey}} {{.dataType}}, sqlStr string, args ...any) (sql.Result, error)
ExecSql(ctx context.Context, {{.lowerStartCamelPrimaryKey}} {{.dataType}}, sqlStr string, args ...any) (sql.Result, error)