package config

// Config is your structure
type Config interface {
	Merge(c any) error
	PostLoad() error
}
