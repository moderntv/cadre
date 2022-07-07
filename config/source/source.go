package source

type Source interface {
	Name() string
	Read() ([]byte, error)
	Load(dst any) error
	Watch() (Watcher, error)
}

type Watcher interface {
	C() chan ConfigChange
	Stop()
}

type ConfigChange struct {
	SourceName string
}
