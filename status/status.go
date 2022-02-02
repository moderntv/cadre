package status

import (
	"errors"
	"sync"
	"time"
)

type ComponentReport struct {
	Status    StatusType `json:"status"`
	Message   string     `json:"message,omitempty"`
	UpdatedAt time.Time  `json:"updated_at,omitempty"`
}
type Report struct {
	Version    string                     `json:"version"`
	Status     StatusType                 `json:"status"`
	Components map[string]ComponentReport `json:"components"`
}

type Status struct {
	version string

	mu         sync.RWMutex
	components map[string]*ComponentStatus
}

var (
	ErrAlreadyExists = errors.New("component already exists")
)

func NewStatus(version string) (status *Status) {
	status = &Status{
		version:    version,
		components: make(map[string]*ComponentStatus),
	}

	return
}

func (s *Status) Register(name string) (cs *ComponentStatus, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.components[name]; ok {
		err = ErrAlreadyExists
		return
	}

	cs = &ComponentStatus{
		status:  ERROR,
		message: "uninitialized",
	}
	s.components[name] = cs

	return
}

func (s *Status) RegisterOrGet(name string) (cs *ComponentStatus, err error) {
	cs, err = s.Register(name)
	if err == nil {
		return
	}
	if err != nil && err != ErrAlreadyExists {
		return
	}
	// err = ErrAlreadyExists
	err = nil

	cs = s.components[name]

	return
}

func (s *Status) Report() (report Report) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	report.Version = s.version
	report.Status = OK
	report.Components = make(map[string]ComponentReport)
	for n, cs := range s.components {
		report.Components[n] = ComponentReport{
			Status:    cs.Status(),
			Message:   cs.Message(),
			UpdatedAt: cs.LastUpdate(),
		}
	}
	for _, s := range report.Components {
		if s.Status == ERROR {
			report.Status = ERROR
			break
		}
		if s.Status == WARN {
			report.Status = WARN
		}
	}

	return
}

type ComponentStatus struct {
	mu        sync.RWMutex
	status    StatusType
	message   string
	updatedAt time.Time
}

func (cs *ComponentStatus) SetStatus(statusType StatusType, message string) {
	cs.mu.Lock()

	cs.message = message
	cs.status = statusType
	cs.updatedAt = time.Now()

	cs.mu.Unlock()
}
func (cs *ComponentStatus) Status() StatusType {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	return cs.status
}
func (cs *ComponentStatus) Message() string {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	return cs.message
}
func (cs *ComponentStatus) LastUpdate() time.Time {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	return cs.updatedAt
}
