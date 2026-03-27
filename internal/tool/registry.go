package tool

type Registry struct {
	handlers map[string]Handler
}

func NewRegistry() *Registry {
	return &Registry{
		handlers: make(map[string]Handler),
	}
}

func (r *Registry) Register(name string, h Handler) {
	r.handlers[name] = h
}

func (r *Registry) Get(name string) (Handler, bool) {
	if v, ok := r.handlers[name]; ok {
		return v, true
	}
	return nil, false
}

func (r *Registry) Names() []string {
	names := []string{}
	for name := range r.handlers {
		names = append(names, name)
	}
	return names
}
