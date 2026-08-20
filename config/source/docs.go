// Package source defines the interfaces a configuration backend implements for
// [github.com/moderntv/cadre/config].
//
// A [Source] can name itself, read its raw payload, decode that payload into a destination struct, and hand
// out a [Watcher]. A Watcher exposes a channel of [ConfigChange] values, each naming the source that
// changed, and a Stop method that ends the watch.
//
// The file backend in [github.com/moderntv/cadre/config/source/file] is the built-in implementation.
// Implement Source to read configuration from somewhere else, such as a key-value store or the environment.
package source
