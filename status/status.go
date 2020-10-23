package status

import (
	"errors"
	"sync"
)

type Status struct {
	version string

	cmu        sync.RWMutex
	components map[string]*ComponentStatus
}

type Report struct {
	Version    string                     `json:"version"`
	Status     StatusType                 `json:"status"`
	Components map[string]ComponentStatus `json:"components"`
}

func NewStatus(version string) (status *Status) {
	status = &Status{
		version:    version,
		components: map[string]*ComponentStatus{},
	}

	return
}

func (s *Status) Register(name string) (cs *ComponentStatus, err error) {
	s.cmu.Lock()
	defer s.cmu.Unlock()
	if _, ok := s.components[name]; ok {
		err = errors.New("component already registered")
		return
	}

	cs = &ComponentStatus{
		Status:  ERROR,
		Message: "uninitialized",
	}
	s.components[name] = cs

	return
}

func (s *Status) Report() (report Report) {
	s.cmu.RLock()
	defer s.cmu.RUnlock()

	report.Version = s.version
	report.Status = OK
	report.Components = map[string]ComponentStatus{}
	for n, cs := range s.components {
		if cs == nil {
			continue
		}

		cs.mu.Lock()
		report.Components[n] = ComponentStatus{
			Status:  cs.Status,
			Message: cs.Message,
		}
		cs.mu.Unlock()
	}
	for _, s := range report.Components {
		if s.Status == ERROR {
			report.Status = ERROR
			break
		}
	}

	return
}

type ComponentStatus struct {
	mu      sync.Mutex
	Status  StatusType `json:"status"`
	Message string     `json:"message"`
}

func (cs *ComponentStatus) SetStatus(statusType StatusType, message string) {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	cs.Message = message
	cs.Status = statusType
}
