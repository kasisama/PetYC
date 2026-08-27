package admin

import (
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// RegisterRoutes 在启动时才会执行，路由冲突会直接 panic 导致进程退出。
// 这里在测试中完整跑一遍注册流程，避免此类问题只能在运行时被发现。
func TestRegisterRoutesMountsConfigCenterAPI(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	RegisterRoutes(router)

	wanted := map[string]string{
		"GET /api/admin/config/schemas":    "",
		"GET /api/admin/config/status":     "",
		"GET /api/admin/config/:schema":    "",
		"PUT /api/admin/config/:schema":    "",
		"PUT /api/admin/groups/bulk-state": "",
		"GET /api/admin/overview":          "",
		"GET /api/admin/players":           "",
		"GET /api/admin/communities":       "",
		"GET /api/admin/platforms/status":  "",
		"GET /api/admin/audit-logs":        "",
		"GET /api/admin/updates/check":     "",
		"GET /api/admin/updates/status":    "",
		"POST /api/admin/updates/install":  "",
	}
	for _, route := range router.Routes() {
		delete(wanted, route.Method+" "+route.Path)
	}
	if len(wanted) > 0 {
		missing := make([]string, 0, len(wanted))
		for route := range wanted {
			missing = append(missing, route)
		}
		t.Fatalf("配置中心接口未挂载到主路由: %s", strings.Join(missing, ", "))
	}
}
