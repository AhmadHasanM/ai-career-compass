// Package httpx berisi format respons error yang dipakai handler dan middleware.
package httpx

import "github.com/gin-gonic/gin"

type ErrorBody struct {
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Fields  map[string]string `json:"fields,omitempty"`
}

// AbortError menulis {"error": {...}} dan menghentikan chain handler.
func AbortError(c *gin.Context, status int, code, message string) {
	c.AbortWithStatusJSON(status, gin.H{"error": ErrorBody{Code: code, Message: message}})
}

func AbortValidation(c *gin.Context, status int, message string, fields map[string]string) {
	c.AbortWithStatusJSON(status, gin.H{"error": ErrorBody{Code: "validation_error", Message: message, Fields: fields}})
}
