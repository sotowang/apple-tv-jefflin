package media

import "context"

type upstreamStatusKey struct{}

// WithUpstreamStatus carries the Archive response status back to the request log.
// A zero value means no upstream request was made (for example, a cache hit).
func WithUpstreamStatus(ctx context.Context) (context.Context, *int) {
	status := new(int)
	return context.WithValue(ctx, upstreamStatusKey{}, status), status
}

func SetUpstreamStatus(ctx context.Context, code int) {
	if status, ok := ctx.Value(upstreamStatusKey{}).(*int); ok {
		*status = code
	}
}
