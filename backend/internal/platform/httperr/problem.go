// Package httperr provides standardized RFC 7807 Problem Details response structures.
package httperr

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// ProblemDetails represents an RFC 7807 compliant problem details JSON object.
type ProblemDetails struct {
	Type     string `json:"type,omitempty"`
	Title    string `json:"title"`
	Status   int    `json:"status"`
	Detail   string `json:"detail,omitempty"`
	Instance string `json:"instance,omitempty"`
	Code     string `json:"code,omitempty"`
	Success  bool   `json:"success"` // Backward compatibility for legacy clients expecting success: false
	Error    string `json:"error"`   // Backward compatibility for legacy clients expecting error: string
}

// NewProblem creates a ProblemDetails struct.
func NewProblem(status int, title, detail, code, instance string) ProblemDetails {
	errText := detail
	if errText == "" {
		errText = title
	}
	return ProblemDetails{
		Type:     "about:blank",
		Title:    title,
		Status:   status,
		Detail:   detail,
		Instance: instance,
		Code:     code,
		Success:  false,
		Error:    errText,
	}
}

// Respond writes an RFC 7807 JSON response to the gin.Context.
func Respond(c *gin.Context, status int, title, detail, code string) {
	instance := ""
	if c.Request != nil && c.Request.URL != nil {
		instance = c.Request.URL.Path
	}
	p := NewProblem(status, title, detail, code, instance)
	c.JSON(status, p)
}

// BadRequest writes a 400 Bad Request problem response.
func BadRequest(c *gin.Context, detail, code string) {
	Respond(c, http.StatusBadRequest, "Bad Request", detail, code)
}

// Unauthorized writes a 401 Unauthorized problem response.
func Unauthorized(c *gin.Context, detail, code string) {
	Respond(c, http.StatusUnauthorized, "Unauthorized", detail, code)
}

// NotFound writes a 404 Not Found problem response.
func NotFound(c *gin.Context, detail, code string) {
	Respond(c, http.StatusNotFound, "Not Found", detail, code)
}

// Unprocessable writes a 422 Unprocessable Entity problem response.
func Unprocessable(c *gin.Context, detail, code string) {
	Respond(c, http.StatusUnprocessableEntity, "Unprocessable Entity", detail, code)
}

// PreconditionFailed writes a 412 Precondition Failed problem response.
func PreconditionFailed(c *gin.Context, detail, code string) {
	Respond(c, http.StatusPreconditionFailed, "Precondition Failed", detail, code)
}

// InternalError writes a 500 Internal Server Error problem response.
func InternalError(c *gin.Context, detail, code string) {
	Respond(c, http.StatusInternalServerError, "Internal Server Error", detail, code)
}

// Internal writes a 500 Internal Server Error problem response with default code.
func Internal(c *gin.Context, detail string) {
	InternalError(c, detail, "")
}

// BadGateway writes a 502 Bad Gateway problem response.
func BadGateway(c *gin.Context, detail, code string) {
	Respond(c, http.StatusBadGateway, "Bad Gateway", detail, code)
}
