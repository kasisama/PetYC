package admin

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const auditRecordedContextKey = "admin_audit_recorded"

func markAuditRecorded(c *gin.Context) {
	if c != nil {
		c.Set(auditRecordedContextKey, true)
	}
}

// AuditWriteRequests guarantees that every protected mutating endpoint leaves
// an audit record. Endpoints with richer before/after auditing can mark the
// context to avoid a duplicate generic entry.
func AuditWriteRequests(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request == nil || c.Request.Method == http.MethodGet || c.Request.Method == http.MethodHead || c.Request.Method == http.MethodOptions {
			c.Next()
			return
		}
		c.Next()
		if db == nil || !db.Migrator().HasTable("admin_audit_logs") {
			return
		}
		if recorded, exists := c.Get(auditRecordedContextKey); exists && recorded == true {
			return
		}
		status := c.Writer.Status()
		path := c.FullPath()
		if strings.TrimSpace(path) == "" && c.Request != nil {
			path = c.Request.URL.Path
		}
		action := strings.ToLower(c.Request.Method) + " " + path
		var operationError error
		if status >= http.StatusBadRequest {
			operationError = fmt.Errorf("HTTP %d", status)
		}
		_ = writeAudit(db, action, "admin_api", path, "后台接口写操作", nil, nil, operationError == nil, operationError)
	}
}
