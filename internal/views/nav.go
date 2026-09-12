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

// isCurrentPage reports whether the rendered page is one of the given paths.
// A pattern ending in a slash (other than "/") matches every page below it.
func isCurrentPage(ctx context.Context, patterns ...string) bool {
	path, _ := ctx.Value(pathKey{}).(string)
	for _, pattern := range patterns {
		if path == pattern || (len(pattern) > 1 && strings.HasSuffix(pattern, "/") && strings.HasPrefix(path, pattern)) {
			return true
		}
	}
	return false
}

// accountLabel is the trigger text of the account menu.
func accountLabel(ctx context.Context) string {
	if name := auth.UserName(ctx); name != "" {
		return name
	}
	return "Account"
}
