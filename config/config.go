package config

// Config is your structure
type Config interface {
	PostLoad() error
}
