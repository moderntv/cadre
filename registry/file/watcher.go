package file

import (
	"github.com/google/uuid"
	"github.com/moderntv/cadre/registry"
)

type watcher struct {
	id       uuid.UUID
	changeCh chan registry.RegistryChange
}

func newWatcher() watcher {
	return watcher{
		id:       uuid.New(),
		changeCh: make(chan registry.RegistryChange),
	}
}
