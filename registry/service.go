package registry

type Service interface {
	Name() string
}

type service struct {
	name string
}

func (s *service) Name() string {
	return s.name
}
