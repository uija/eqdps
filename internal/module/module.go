package module

type Module interface {
	Init(*Context, func()) error
	Shutdown()
}
