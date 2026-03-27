package tool

import (
	"context"
	"fmt"
)

type Dispatcher struct {
	reg *Registry
}

func NewDispatcher(reg *Registry) *Dispatcher {
	return &Dispatcher{
		reg: reg,
	}
}

func (d *Dispatcher) Process(ctx context.Context, name string, input map[string]any) string {
	if d == nil || d.reg == nil {
		return "Error: tool dispatcher is not initialized"
	}

	handler, ok := d.reg.Get(name)
	if !ok {
		return fmt.Sprintf("Error: Unknown tool '%s'", name)
	}

	result, err := handler.Call(ctx, input)
	if err != nil {
		return fmt.Sprintf("Error: %v", err)
	}

	return result
}
