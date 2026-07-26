package module

type Module interface {
	Init(*Context) error
	Shutdown()
}
