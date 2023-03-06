package environment

import (
	"fmt"
	"os"
	"reflect"
	"strings"

	"github.com/moderntv/cadre/config/encoder"
	"github.com/moderntv/cadre/config/source"
	"github.com/spf13/viper"
)

var (
	Name               = "environment"
	_    source.Source = &EnvironmentSource{}
)

type EnvironmentSource struct {
	prefix  string
	encoder encoder.Encoder
	viper   *viper.Viper
}

func NewSource(prefix string, encoder encoder.Encoder, v *viper.Viper) (es *EnvironmentSource, err error) {
	es = &EnvironmentSource{
		prefix:  prefix,
		encoder: encoder,
		viper:   v,
	}
	es.viper.SetEnvPrefix(strings.ToUpper(prefix))
	es.viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	return
}

func (es *EnvironmentSource) Name() string {
	return Name
}

func (es *EnvironmentSource) bindEnvs(configStructPtr interface{}, parts ...string) {
	configStructValue := reflect.ValueOf(configStructPtr).Elem()
	configStructType := configStructValue.Type()

	for _, field := range reflect.VisibleFields(configStructType) {
		if !field.IsExported() || field.Anonymous {
			continue
		}

		fieldValue := configStructValue.FieldByName(field.Name)
		fieldType, ok := configStructType.FieldByName(field.Name)
		if !ok {
			continue
		}

		var tag string
		tagRaw := fieldType.Tag.Get("mapstructure")
		tagParts := strings.Split(tagRaw, ",")
		if len(tagParts) == 0 {
			continue
		}

		tag = tagParts[0]
		if tag == "-" {
			continue
		}

		switch fieldValue.Kind() { // nolint:exhaustive
		case reflect.Struct:
			es.bindEnvs(fieldValue.Addr().Interface(), append(parts, tag)...)
		default:
			_ = es.viper.BindEnv(strings.Join(append(parts, tag), "."))
		}
	}
}

func (es *EnvironmentSource) Read() (d []byte, err error) {
	for _, env := range os.Environ() {
		pair := strings.SplitN(env, "=", 2)
		if !strings.HasPrefix(pair[0], es.prefix) {
			continue
		}

		d = []byte(env)
	}

	return
}

func (es *EnvironmentSource) Load(dst any) (err error) {
	d, err := es.Read()
	if err != nil {
		return fmt.Errorf("data read failed: %w", err)
	}

	return es.encoder.Decode(d, dst)
}

func (es *EnvironmentSource) Watch() (w source.Watcher, err error) {
	w = newWatcher(es.prefix)
	return
}
