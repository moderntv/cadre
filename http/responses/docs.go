// Package responses writes the JSON envelope Cadre services answer with.
//
// Successful payloads are wrapped in a "data" key and failures in an "errors" array, so that clients can
// rely on one shape:
//
//	responses.Ok(c, order)                       // 200 {"data": {...}}
//	responses.OkWithMeta(c, orders, pagination)  // 200 {"data": [...], "metadata": {...}}
//	responses.Created(c, order)                  // 201
//	responses.NotFound(c, responses.NewError(err))
//
// Helpers exist for the status codes a service usually needs: [Ok], [OkWithMeta], [Created],
// [BadRequest], [CannotBind], [Unauthorized], [Forbidden], [NotFound], [Timeout], [Conflict],
// [InternalError] and [Unavailable]. Each one aborts the gin context, so a handler should return
// immediately afterwards.
//
// # Mapping domain errors
//
// [FromError] translates the sentinel error types of [github.com/moderntv/cadre/errors] into status codes,
// which keeps that mapping out of individual handlers:
//
//	order, err := repo.Find(c.Param("id"))
//	if err != nil {
//		responses.FromError(c, err) // ErrNotFound -> 404, ErrInvalidInput -> 400, ...
//		return
//	}
//
//	responses.Ok(c, order)
//
// Anything FromError does not recognise becomes a 500.
package responses
