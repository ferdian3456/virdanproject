package model

import "net/http"

type ApiError interface {
	error
	StatusCode() int
	GetCode() string
	GetParam() string
}

type BadRequestError struct {
	Code    string
	Message string
	Param   string
}

func (e *BadRequestError) Error() string {
	return e.Message
}

func (e *BadRequestError) StatusCode() int {
	return http.StatusBadRequest
}

func (e *BadRequestError) GetCode() string {
	return e.Code
}

func (e *BadRequestError) GetParam() string {
	return e.Param
}

type NotFoundError struct {
	Code    string
	Message string
	Param   string
}

func (e *NotFoundError) Error() string {
	return e.Message
}

func (e *NotFoundError) StatusCode() int {
	return http.StatusNotFound
}

func (e *NotFoundError) GetCode() string {
	return e.Code
}

func (e *NotFoundError) GetParam() string {
	return e.Param
}

type UnauthorizedError struct {
	Code    string
	Message string
	Param   string
}

func (e *UnauthorizedError) Error() string {
	return e.Message
}

func (e *UnauthorizedError) StatusCode() int {
	return http.StatusUnauthorized
}

func (e *UnauthorizedError) GetCode() string {
	return e.Code
}

func (e *UnauthorizedError) GetParam() string {
	return e.Param
}
