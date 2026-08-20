// Package errors provides the error vocabulary that lets a domain layer describe what went wrong without
// knowing how it will be reported.
//
// The package exposes a small set of sentinel types - [ErrInvalidInput], [ErrNotAllowed], [ErrNotFound],
// [ErrTemporaryUnavailable] and [ErrInternalError]. Wrap a cause with [NewTyped] to tag it with one of
// them:
//
//	func (r *repository) Find(id string) (*Order, error) {
//		// ...
//		return nil, errors.NewTyped(errors.ErrNotFound, fmt.Errorf("order %q does not exist", id))
//	}
//
// The resulting [TypedError] keeps the cause as its message and returns the sentinel from Unwrap, so
// standard errors.Is reports the category while the message still describes the specific failure.
//
// The transport layer then maps the category without any knowledge of the domain - see
// [github.com/moderntv/cadre/http/responses.FromError], which turns each sentinel into the matching HTTP
// status code.
package errors
