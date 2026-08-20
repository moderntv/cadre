// Package config loads an application's configuration from an ordered list of sources and can notify the
// application when one of them changes.
//
//	src, err := file.NewSource("./config.yaml", yaml.NewEncoder())
//	if err != nil {
//		return err
//	}
//
//	manager, err := config.NewManager(config.WithSource(src))
//	if err != nil {
//		return err
//	}
//
//	var cfg Config
//
//	err = manager.Load(&cfg)
//
// [Manager.Load] applies the sources in the order they were added with [WithSource], decoding each one into
// the same destination, so a later source overrides the values of an earlier one. A source that fails to
// load aborts the whole call.
//
// # Watching for changes
//
// [Manager.Subscribe] returns a channel that receives a [github.com/moderntv/cadre/config/source.ConfigChange]
// whenever a watched source changes, naming the source that changed; reload by calling Load again. The
// channel is buffered and sends to it are non-blocking, so a subscriber that stops reading loses changes
// rather than stalling the manager. Release it with [Manager.Unsubscribe].
//
// # The Config interface
//
// [Config] describes a configuration struct that can post-process itself after loading - validating values
// or filling in derived fields. Note that Load takes an any and does not call PostLoad for you, so invoke
// it yourself once loading has succeeded:
//
//	err = manager.Load(&cfg)
//	if err != nil {
//		return err
//	}
//
//	err = cfg.PostLoad()
//
// Sources are described by [github.com/moderntv/cadre/config/source.Source] and the encoding of their
// payload by [github.com/moderntv/cadre/config/encoder.Encoder].
package config
