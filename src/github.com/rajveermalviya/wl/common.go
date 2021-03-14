package wl

type ProxyID uint32

type Dispatcher interface {
	Dispatch(*Event)
}

type Proxy interface {
	Context() *Context
	SetContext(ctx *Context)
	ID() ProxyID
	SetID(id ProxyID)
}

type BaseProxy struct {
	id  ProxyID
	ctx *Context
}

func (p *BaseProxy) ID() ProxyID {
	return p.id
}

func (p *BaseProxy) SetID(id ProxyID) {
	p.id = id
}

func (p *BaseProxy) Context() *Context {
	return p.ctx
}

func (p *BaseProxy) SetContext(ctx *Context) {
	p.ctx = ctx
}

type Handler interface {
	Handle(ev interface{})
}

type eventHandler struct {
	f func(interface{})
}

func HandlerFunc(f func(interface{})) Handler {
	return &eventHandler{f}
}

func (h *eventHandler) Handle(ev interface{}) {
	h.f(ev)
}
