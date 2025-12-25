package utils

import "context"

func GetAccount(ctx context.Context) string {
	account, ok := ctx.Value("account").(string)
	if !ok || account == "" {
		account = "system"
	}
	return account
}
