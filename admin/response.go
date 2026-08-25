package admin

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func responseRequestID(c *gin.Context) string {
	if value, exists := c.Get("request_id"); exists {
		if requestID, ok := value.(string); ok && requestID != "" {
			return requestID
		}
	}
	requestID := ""
	if c.Request != nil {
		requestID = strings.TrimSpace(c.GetHeader("X-Request-ID"))
	}
	if requestID == "" || len(requestID) > 128 {
		requestID = uuid.NewString()
	}
	c.Set("request_id", requestID)
	c.Header("X-Request-ID", requestID)
	return requestID
}

// Success 返回 HTTP 200 及标准的 {code: 0, msg: "success", data} JSON 格式
func Success(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, gin.H{
		"code":       0,
		"msg":        "success",
		"data":       data,
		"request_id": responseRequestID(c),
	})
}

// Error keeps the standard JSON envelope while also returning an HTTP status
// that reflects the error class. Clients can rely on either layer.
func Error(c *gin.Context, code int, msg string) {
	status := http.StatusBadRequest
	switch {
	case code == codeSchemaNotFound:
		status = http.StatusNotFound
	case code >= 4090 && code < 4100:
		status = http.StatusConflict
	case code >= codeInternalError:
		status = http.StatusInternalServerError
	}
	c.JSON(status, gin.H{
		"code":       code,
		"msg":        msg,
		"data":       nil,
		"request_id": responseRequestID(c),
	})
}
