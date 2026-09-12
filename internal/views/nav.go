package views

import (
	"context"
	"strings"

	"camplist/internal/auth"
)

type pathKey struct{}

// WithPath records the request path so the header can mark the current
// section. The web package sets it when rendering a page.
func WithPath(ctx context.Context, path string) context.Context {
	return context.WithValue(ctx, pathKey{}, path)
}

// isCurrentPage reports whether the page at exact, or any page under prefix,
// is the one being rendered.
func isCurrentPage(ctx context.Context, exact string, prefix string) bool {
	path, _ := ctx.Value(pathKey{}).(string)
	return path == exact || (prefix != "" && strings.HasPrefix(path, prefix))
}

// accountLabel is the trigger text of the account menu.
func accountLabel(ctx context.Context) string {
	if name := auth.UserName(ctx); name != "" {
		return name
	}
	return "Account"
}
