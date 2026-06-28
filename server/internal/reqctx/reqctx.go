package reqctx

import (
	"context"

	"github.com/paleicikas/importinvoices/server/internal/domain"
)

type key int

const (
	userKey key = iota
	orgKey
	csrfKey
)

func WithUser(ctx context.Context, u *domain.User) context.Context {
	return context.WithValue(ctx, userKey, u)
}

func User(ctx context.Context) (*domain.User, bool) {
	u, ok := ctx.Value(userKey).(*domain.User)
	return u, ok
}

func WithOrganization(ctx context.Context, org *domain.Organization) context.Context {
	return context.WithValue(ctx, orgKey, org)
}

func Organization(ctx context.Context) (*domain.Organization, bool) {
	org, ok := ctx.Value(orgKey).(*domain.Organization)
	return org, ok
}

func WithCSRFToken(ctx context.Context, token string) context.Context {
	return context.WithValue(ctx, csrfKey, token)
}

func CSRFToken(ctx context.Context) (string, bool) {
	token, ok := ctx.Value(csrfKey).(string)
	return token, ok
}
