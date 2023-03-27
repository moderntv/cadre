package source

import "context"

type Source interface {
	Name() string
	Read() ([]byte, error)
	Load(dst any) error
	Save(dst any) error
	Watch() (Watcher, error)
}

type Watcher interface {
	C(ctx context.Context) chan ConfigChange
}

type ConfigChange struct {
	SourceName string
	Type       string
}
