package config

// Config represents config structure.
type Config interface {
	PostLoad() error
}
