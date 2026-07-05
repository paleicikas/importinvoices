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

// IsAdmin reports whether the context carries an admin user. Returns false if
// no user is present or the user is not an admin.
func IsAdmin(ctx context.Context) bool {
	u, ok := User(ctx)
	return ok && u != nil && u.Role == domain.RoleAdmin
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
