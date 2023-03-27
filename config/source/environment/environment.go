package environment

import (
	"os"
	"reflect"
	"strings"

	"github.com/moderntv/cadre/config/source"
	"github.com/spf13/viper"
)

var (
	Name               = "environment"
	_    source.Source = &EnvironmentSource{}
)

type EnvironmentSource struct {
	prefix string
	viper  *viper.Viper
}

func NewSource(prefix string, v *viper.Viper) (es *EnvironmentSource, err error) {
	es = &EnvironmentSource{
		prefix: prefix,
		viper:  v,
	}
	es.viper.SetEnvPrefix(strings.ToUpper(prefix))
	es.viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	return
}

func (es *EnvironmentSource) Name() string {
	return Name
}

func (es *EnvironmentSource) bindEnvs(configStructPtr interface{}, parts ...string) error {
	configStructValue := reflect.ValueOf(configStructPtr).Elem()
	configStructType := configStructValue.Type()

	var err error
	for _, field := range reflect.VisibleFields(configStructType) {
		if err != nil {
			continue
		}

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
			err = es.viper.BindEnv(strings.Join(append(parts, tag), "."))
		}
	}

	return err
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

func (es *EnvironmentSource) Save(dst any) (err error) {
	return
}

func (es *EnvironmentSource) Load(dst any) (err error) {
	err = es.bindEnvs(dst)
	if err != nil {
		return
	}

	es.viper.Unmarshal(dst)
	return nil
}

func (es *EnvironmentSource) Watch() (w source.Watcher, err error) {
	check := func(c chan source.ConfigChange) {
		d, err := es.Read()
		if err != nil {
			return
		}

		if len(d) != 0 {
			c <- source.ConfigChange{
				SourceName: es.Name(),
			}
		}
	}

	w = newWatcher(es.prefix, check)
	return
}
