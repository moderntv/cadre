// Package file implements a [github.com/moderntv/cadre/config/source.Source] that reads configuration from
// a file on disk.
//
//	src, err := file.NewSource("./config.yaml", yaml.NewEncoder())
//
// The encoder decides how the file's contents are decoded, so the same source works for YAML, JSON or any
// other format with an [github.com/moderntv/cadre/config/encoder.Encoder] implementation. The file is read
// on every Load rather than cached, and its watcher reports a change whenever the file is written.
package file
