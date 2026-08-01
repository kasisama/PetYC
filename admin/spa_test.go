package admin

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// Vue 后台使用 history 模式路由，用户在 /admin/config 刷新页面时浏览器会直接请求该路径。
// 静态文件服务找不到对应文件会返回 404，因此需要回退到 index.html 交给前端路由处理。
func TestAdminDeepLinkFallsBackToIndex(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	RegisterRoutes(router)

	cases := []string{"/admin", "/admin/", "/admin/config", "/admin/config/system"}
	for _, path := range cases {
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))

		if recorder.Code != http.StatusOK {
			t.Fatalf("%s 未返回 200，实际 %d", path, recorder.Code)
		}
		if !strings.Contains(recorder.Body.String(), `<div id="app">`) {
			t.Fatalf("%s 未返回 SPA 入口页面，实际内容: %s", path, recorder.Body.String())
		}
	}
}

// 真实存在的静态资源必须原样返回，不能被回退逻辑吞掉。
func TestAdminServesRealAssets(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	RegisterRoutes(router)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/admin/favicon.svg", nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("静态资源未返回 200，实际 %d", recorder.Code)
	}
	if strings.Contains(recorder.Body.String(), `<div id="app">`) {
		t.Fatal("静态资源被错误地回退成了 index.html")
	}
}

// 未知的 /api 路径不应该被 SPA 回退逻辑接管，否则前端会把 HTML 当 JSON 解析。
func TestUnknownAPIPathDoesNotFallBackToIndex(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	RegisterRoutes(router)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/admin/not-exist", nil))

	if strings.Contains(recorder.Body.String(), `<div id="app">`) {
		t.Fatal("未知 API 路径返回了 SPA 页面")
	}
}
