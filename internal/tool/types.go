package tool

import "context"

type Call struct {
	Name  string
	Input map[string]any
}

type Handler interface {
	Call(ctx context.Context, input map[string]any) (string, error)
}

type HandlerFunc func(ctx context.Context, input map[string]any) (string, error)

func (f HandlerFunc) Call(ctx context.Context, input map[string]any) (string, error) {
	return f(ctx, input)
}
