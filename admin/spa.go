package admin

import (
	"io/fs"
	"net/http"
	"path"
	"strings"

	"github.com/gin-gonic/gin"
)

// newAdminUIHandler 返回一个服务 Vue 后台单页应用的处理函数。
//
// Vue Router 使用 history 模式，/admin/config 这类路径在磁盘上并不存在对应文件。
// 用户直接访问或刷新这些地址时，必须回退到 index.html 由前端路由接管，
// 否则会得到 404。真实存在的静态资源仍然原样返回。
func newAdminUIHandler(assets fs.FS) gin.HandlerFunc {
	fileServer := http.FileServer(http.FS(assets))

	return func(c *gin.Context) {
		// gin 的 *filepath 通配符会带上前导斜杠，且 /admin 本身没有该参数。
		requested := strings.TrimPrefix(c.Param("filepath"), "/")

		if requested != "" && assetExists(assets, requested) {
			// http.FileServer 以 URL 路径查找文件，需要剥掉 /admin 前缀。
			c.Request.URL.Path = "/" + requested
			fileServer.ServeHTTP(c.Writer, c.Request)
			return
		}

		serveAdminIndex(c, assets)
	}
}

// assetExists 判断请求的路径是否对应一个真实的嵌入文件。
// 目录不算命中，因为后台没有目录列表需求，一律回退到 index.html。
func assetExists(assets fs.FS, name string) bool {
	cleaned := path.Clean(name)
	if cleaned == "." || strings.HasPrefix(cleaned, "..") {
		return false
	}
	info, err := fs.Stat(assets, cleaned)
	return err == nil && !info.IsDir()
}

func serveAdminIndex(c *gin.Context, assets fs.FS) {
	content, err := fs.ReadFile(assets, "index.html")
	if err != nil {
		c.String(http.StatusInternalServerError, "后台页面资源缺失")
		return
	}
	// 入口页面引用的是带哈希的资源文件名，不能被缓存，否则前端更新后无法生效。
	c.Header("Cache-Control", "no-cache")
	c.Data(http.StatusOK, "text/html; charset=utf-8", content)
}
