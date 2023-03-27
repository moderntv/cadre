package config

type BaseConfig struct {
	Type     string `mapstructure:"type"`
	FilePath string `mapstructure:"file_path"`
}

func (b *BaseConfig) PostLoad() error {
	return nil
}

func (b *BaseConfig) Merge(c any) error {
	m := c.(*BaseConfig)
	b.Type = m.Type
	b.FilePath = m.FilePath
	return nil
}
