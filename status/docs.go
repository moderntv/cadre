// Package status tracks the health of an application's components and renders it as a report.
//
// A [Status] owns a set of named components, each represented by a [ComponentStatus] that the owning code
// updates as it learns about its own health:
//
//	appStatus := status.NewStatus("1.0.0")
//
//	dbStatus, err := appStatus.Register("database")
//	if err != nil {
//		return err
//	}
//
//	dbStatus.SetStatus(status.OK, "connected")
//
// Components start out as [ERROR] with the message "uninitialized", so a component nobody reports on keeps
// the application unhealthy rather than silently passing. [Status.Register] refuses a name that is already
// taken with [ErrAlreadyExists]; use [Status.RegisterOrGet] when several call sites may register the same
// component.
//
// [Status.Report] aggregates the components into a [Report]. The overall status is the worst component
// status, ordered [OK], [WARN], [ERROR]. Cadre serves the report on /status and answers 503 as soon as the
// overall status is ERROR, which makes the endpoint usable directly as a readiness probe. When the gRPC
// health service is enabled, Cadre also polls the report every five seconds and moves the health server
// between serving and not-serving to match.
//
// [Status] and [ComponentStatus] are safe for concurrent use. [StatusType] marshals to and from its name
// ("OK", "WARN", "ERROR") in JSON rather than to its numeric value.
package status
