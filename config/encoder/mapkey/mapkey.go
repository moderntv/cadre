package mapkey

import (
	"github.com/moderntv/cadre/config/encoder"
	"gopkg.in/yaml.v2"
)

var _ encoder.Encoder = &MapKeyEncoder{}

type MapKeyEncoder struct{}

func NewEncoder() (mke *MapKeyEncoder) {
	return &MapKeyEncoder{}
}

func (mke *MapKeyEncoder) Encode(data interface{}) ([]byte, error) {
	return yaml.Marshal(data)
}

func (mke *MapKeyEncoder) Decode(data []byte, dst interface{}) error {
	return yaml.Unmarshal(data, dst)
}
