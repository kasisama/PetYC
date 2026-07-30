package admin

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Success 返回 HTTP 200 及标准的 {code: 0, msg: "success", data} JSON 格式
func Success(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"msg":  "success",
		"data": data,
	})
}

// Error 返回 HTTP 200 及标准的 {code, msg, data: nil} JSON 格式表示业务异常
func Error(c *gin.Context, code int, msg string) {
	c.JSON(http.StatusOK, gin.H{
		"code": code,
		"msg":  msg,
		"data": nil,
	})
}
