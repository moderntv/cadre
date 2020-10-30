package responses

import (
	"errors"

	"github.com/gin-gonic/gin"

	cerr "github.com/moderntv/cadre/errors"
)

type SuccessResponse struct {
	Data gin.H `json:"data"`
}

type Error struct {
	Type    string      `json:"type,omitempty"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

type ErrorResponse struct {
	Message string  `json:"message,omitempty"`
	Errors  []Error `json:"errors,omitempty"`
}

func NewErrors(errs ...error) []Error {
	errors := make([]Error, len(errs))
	for i, err := range errs {
		errors[i] = NewError(err)
	}
	return errors
}

func NewError(err error) Error {
	return Error{
		Type:    "GENERIC_ERROR",
		Message: "Error encountered",
		Data:    err.Error(),
	}
}

func FromError(c *gin.Context, err error) {
	if errors.Is(err, cerr.ErrInvalidInput) {
		BadRequest(c, NewError(err))
		return
	}
	if errors.Is(err, cerr.ErrNotAllowed) {
		Forbidden(c, NewError(err))
		return
	}
	if errors.Is(err, cerr.ErrNotFound) {
		NotFound(c, NewError(err))
		return
	}
	if errors.Is(err, cerr.ErrTemporyUnavailable) {
		Unavailable(c, NewError(err))
		return
	}
	if errors.Is(err, cerr.ErrInternalError) {
		InternalError(c, NewError(err))
		return
	}
	InternalError(c, NewError(err))
}

func Ok(c *gin.Context, data interface{}) {
	c.AbortWithStatusJSON(200, gin.H{
		"data": data,
	})
}

// OkWithMeta sets the HTTP response status to 200
func OkWithMeta(c *gin.Context, data interface{}, metadata interface{}) {
	c.AbortWithStatusJSON(200, gin.H{
		"data":     data,
		"metadata": metadata,
	})
}

// Created sets the HTTP response status to 201
func Created(c *gin.Context, data interface{}) {
	c.AbortWithStatusJSON(201, gin.H{
		"data": data,
	})
}

// BadRequest sets the HTTP response status to 400
func BadRequest(c *gin.Context, errors ...Error) {
	c.AbortWithStatusJSON(400, gin.H{
		"message": "The request is not valid in this context",
		"errors":  errors,
	})
}

// CannotBind sets the HTTP response status to 400
func CannotBind(c *gin.Context, err error) {
	var (
		msg  = "Sent data do not correspond to the template"
		data string
	)
	if err != nil {
		msg = "There were errors when applying sent data to template: " + err.Error()
		data = err.Error()
	}
	BadRequest(c, Error{
		Type:    "UNKNOWN_INPUT_VALIDATION_ERROR",
		Message: msg,
		Data:    data,
	})
}

// Unauthorized sets the HTTP response status to 401
func Unauthorized(c *gin.Context, errors ...Error) {
	c.AbortWithStatusJSON(401, gin.H{
		"message": "You have to be logged in to view this resource",
		"errors":  errors,
	})
}

// Forbidden sets the HTTP response status to 403
func Forbidden(c *gin.Context, errors ...Error) {
	c.AbortWithStatusJSON(403, gin.H{
		"message": "You are not allowed to view this resource",
		"errors":  errors,
	})
}

// NotFound sets the HTTP response status to 404
func NotFound(c *gin.Context, errors ...Error) {
	c.AbortWithStatusJSON(404, gin.H{
		"message": "The resource is unavailable",
		"errors":  errors,
	})
}

// Timeout sets the HTTP response status to 408
func Timeout(c *gin.Context) {
	c.AbortWithStatusJSON(408, gin.H{
		"message": "Request timed out",
	})
}

// Conflict sets the HTTP response status to 409
func Conflict(c *gin.Context, errors ...Error) {
	c.AbortWithStatusJSON(409, gin.H{
		"message": "Cannot complete due to a conflict",
		"errors":  errors,
	})
}

// InternalError sets the HTTP response status to 500
func InternalError(c *gin.Context, errors ...Error) {
	c.AbortWithStatusJSON(500, gin.H{
		"message": "An unexpected error has occured. A team of monkeys was already sent to site. " +
			"We're not sure, when it will be ready, but it sure as hell will be banana",
		"errors": errors,
	})
}

// Unavailable sets the HTTP response status to 503
func Unavailable(c *gin.Context, errors ...Error) {
	c.AbortWithStatusJSON(503, gin.H{
		"message": "Service is temporarily unavailable",
		"errors":  errors,
	})
}
