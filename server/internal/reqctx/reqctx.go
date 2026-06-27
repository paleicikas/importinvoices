package reqctx

import (
	"context"

	"github.com/paleicikas/importinvoices/server/internal/domain"
)

type key int

const (
	userKey key = iota
	orgKey
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
