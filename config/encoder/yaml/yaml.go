package yaml

import (
	"github.com/moderntv/cadre/config/encoder"
	"gopkg.in/yaml.v2"
)

var _ encoder.Encoder = &YamlEncoder{}

type YamlEncoder struct{}

func NewEncoder() (ye *YamlEncoder) {
	return &YamlEncoder{}
}

func (ye *YamlEncoder) Encode(data interface{}) ([]byte, error) {
	return yaml.Marshal(data)
}

func (ye *YamlEncoder) Decode(data []byte, dst interface{}) error {
	return yaml.Unmarshal(data, dst)
}
