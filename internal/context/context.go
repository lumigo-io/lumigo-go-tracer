package context

import "context"

// An unexported type to be used as the key for types in this package.
// This prevents collisions with keys defined in other packages.
type key struct{}

// The key for a LumigoContext in Contexts
var lumigoKey = &key{}

// LumigoContext is the set of metadata that is passed for every Invoke.
type LumigoContext struct {
	TracerVersion string
}

// NewContext returns a new Context that carries value lumigo context.
func NewContext(parent context.Context, lc *LumigoContext) context.Context {
	return context.WithValue(parent, lumigoKey, lc)
}

// FromContext returns the lumigoKey value stored in ctx, if any.
func FromContext(ctx context.Context) (*LumigoContext, bool) {
	lc, ok := ctx.Value(lumigoKey).(*LumigoContext)
	return lc, ok
}
