package admin

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"qq-pet-saas/database"
	"qq-pet-saas/models"
	"qq-pet-saas/security"
)

const (
	maxUploadBytes = 10 * 1024 * 1024
	maxImagePixels = 40_000_000
)

func RegisterRoutes(router *gin.Engine) {
	subFS, err := fs.Sub(Assets, "dist")
	if err != nil {
		log.Fatalf("创建后台静态资源失败: %v", err)
	}
	serveAdminUI := newAdminUIHandler(subFS)
	router.GET("/admin", serveAdminUI)
	router.HEAD("/admin", serveAdminUI)
	router.GET("/admin/*filepath", serveAdminUI)
	router.HEAD("/admin/*filepath", serveAdminUI)

	if _, err = security.LoadCredentials(); err != nil {
		log.Fatalf("加载管理员凭据失败: %v", err)
	}
	sessions := NewSessionManager()
	auth := NewAuthHandler(sessions)
	authRoutes := router.Group("/api/admin/auth", RequireSameOrigin())
	authRoutes.POST("/login", auth.Login)
	authRoutes.POST("/setup", auth.SetupPassword)
	authRoutes.GET("/session", auth.Session)

	protected := router.Group("/api/admin", RequireAdminSession(sessions), RequireSameOrigin(), AuditWriteRequests(database.DB))
	protected.POST("/auth/logout", auth.Logout)
	protected.PUT("/auth/password", auth.ChangePassword)

	api := protected.Group("", ProtectConfigReads())
	api.GET("/groups", GetGroups)
	api.PUT("/groups/:id", UpdateGroup)
	api.PUT("/groups/bulk-state", BulkUpdateGroups)
	api.DELETE("/groups/:id", DeleteGroup)
	api.POST("/upload", UploadImage)
	RegisterConfigRoutes(api, NewConfigAPI(database.DB))
	RegisterProfileRoutes(api, database.DB)
	RegisterOnboardingRoutes(api)
	RegisterContentRoutes(api, database.DB)
	RegisterEcosystemRoutes(api, database.DB)
	RegisterAdventureRoutes(api, database.DB)
}

func GetGroups(c *gin.Context) {
	var groups []models.GroupSwitch
	if err := database.DB.Order("group_id asc").Find(&groups).Error; err != nil {
		Error(c, codeInternalError, "读取群组状态失败")
		return
	}
	Success(c, groups)
}

func UpdateGroup(c *gin.Context) {
	groupID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		Error(c, codeInvalidPayload, "群号格式不正确")
		return
	}
	type request struct {
		GroupName *string `json:"group_name"`
		IsActive  *bool   `json:"is_active"`
	}
	var payload request
	if err = c.ShouldBindJSON(&payload); err != nil {
		Error(c, codeInvalidPayload, "请求体不是合法的 JSON")
		return
	}
	group := models.GroupSwitch{GroupID: groupID, Platform: "onebot", SpaceID: strconv.FormatInt(groupID, 10), IsActive: true}
	lookup := database.DB.Limit(1).Find(&group, "group_id = ?", groupID)
	if lookup.Error != nil {
		Error(c, codeInternalError, "读取群组状态失败")
		return
	}
	if payload.GroupName != nil {
		group.GroupName = *payload.GroupName
	}
	if payload.IsActive != nil {
		group.IsActive = *payload.IsActive
	}
	if err = database.DB.Save(&group).Error; err != nil {
		Error(c, codeInternalError, "保存群组状态失败")
		return
	}
	Success(c, group)
}

func BulkUpdateGroups(c *gin.Context) {
	type request struct {
		GroupIDs json.RawMessage `json:"group_ids"`
		IsActive *bool           `json:"is_active"`
	}
	var payload request
	if err := c.ShouldBindJSON(&payload); err != nil {
		Error(c, codeInvalidPayload, "请求体不是合法的 JSON")
		return
	}
	if payload.IsActive == nil || len(payload.GroupIDs) == 0 {
		Error(c, codeInvalidPayload, "group_ids 和 is_active 为必填项")
		return
	}
	allGroups := bytes.Equal(bytes.TrimSpace(payload.GroupIDs), []byte("null"))
	groupIDs := make([]int64, 0)
	if !allGroups {
		if err := json.Unmarshal(payload.GroupIDs, &groupIDs); err != nil || len(groupIDs) == 0 {
			Error(c, codeInvalidPayload, "group_ids 必须是非空群号数组，或使用 null 表示全部群组")
			return
		}
	}
	query := database.DB.Model(&models.GroupSwitch{})
	if allGroups {
		query = query.Where("1 = 1")
	} else {
		query = query.Where("group_id IN ?", groupIDs)
	}
	result := query.Update("is_active", *payload.IsActive)
	if result.Error != nil {
		Error(c, codeInternalError, "批量更新群组状态失败")
		return
	}
	var groups []models.GroupSwitch
	if err := database.DB.Order("group_id asc").Find(&groups).Error; err != nil {
		Error(c, codeInternalError, "读取群组状态失败")
		return
	}
	Success(c, gin.H{"updated": result.RowsAffected, "groups": groups})
}

func DeleteGroup(c *gin.Context) {
	groupID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		Error(c, codeInvalidPayload, "群号格式不正确")
		return
	}
	result := database.DB.Delete(&models.GroupSwitch{}, "group_id = ?", groupID)
	if result.Error != nil {
		Error(c, codeInternalError, "删除群组记录失败")
		return
	}
	Success(c, gin.H{"deleted": result.RowsAffected})
}

func UploadImage(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		Error(c, codeInvalidPayload, "请选择需要上传的图片")
		return
	}
	if file.Size > maxUploadBytes {
		Error(c, codeInvalidPayload, "文件大小不能超过 10MB")
		return
	}
	source, err := file.Open()
	if err != nil {
		Error(c, codeInvalidPayload, "图片读取失败")
		return
	}
	extension, width, height, validationErr := inspectImageUpload(source)
	_ = source.Close()
	if validationErr != nil {
		Error(c, codeInvalidPayload, validationErr.Error())
		return
	}
	uploadDir := filepath.Join("上传")
	publicDir := filepath.Join(".", "图片", uploadDir)
	if err = os.MkdirAll(publicDir, 0755); err != nil {
		Error(c, codeInternalError, "创建上传目录失败")
		return
	}
	uniqueName := fmt.Sprintf("%d%s", time.Now().UnixNano(), extension)
	publicPath := filepath.Join(publicDir, uniqueName)
	if err = c.SaveUploadedFile(file, publicPath); err != nil {
		Error(c, codeInternalError, "保存上传图片失败")
		return
	}
	finalPath := filepath.Join(uploadDir, uniqueName)
	asset := models.ImageAsset{
		ID: uuid.NewString(), OriginalName: filepath.Base(file.Filename), StoredName: uniqueName,
		StoredPath: finalPath, URL: "/images/上传/" + uniqueName, MIMEType: imageMIMEType(extension),
		Size: file.Size, Width: width, Height: height, CreatedAt: time.Now(),
	}
	if database.DB == nil || database.DB.Create(&asset).Error != nil {
		_ = os.Remove(publicPath)
		Error(c, codeInternalError, "登记图片资产失败")
		return
	}
	Success(c, gin.H{"message": "图片上传成功", "asset": asset, "path": asset.StoredPath, "url": asset.URL})
}

func validateImageUpload(reader io.Reader) (string, error) {
	extension, _, _, err := inspectImageUpload(reader)
	return extension, err
}

func inspectImageUpload(reader io.Reader) (string, int, int, error) {
	data, err := io.ReadAll(io.LimitReader(reader, maxUploadBytes+1))
	if err != nil {
		return "", 0, 0, err
	}
	if len(data) == 0 || len(data) > maxUploadBytes {
		return "", 0, 0, errors.New("文件为空或超过大小限制")
	}
	if width, height, ok := webpDimensions(data); ok {
		if int64(width)*int64(height) > maxImagePixels {
			return "", 0, 0, errors.New("图片像素超出限制")
		}
		return ".webp", width, height, nil
	}
	decoded, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return "", 0, 0, errors.New("仅支持 JPEG、PNG、GIF、WebP 图片")
	}
	if decoded.Width <= 0 || decoded.Height <= 0 || int64(decoded.Width)*int64(decoded.Height) > maxImagePixels {
		return "", 0, 0, errors.New("图片尺寸或像素超出限制")
	}
	switch format {
	case "jpeg":
		return ".jpg", decoded.Width, decoded.Height, nil
	case "png":
		return ".png", decoded.Width, decoded.Height, nil
	case "gif":
		return ".gif", decoded.Width, decoded.Height, nil
	default:
		return "", 0, 0, errors.New("不支持的图片格式")
	}
}

func imageMIMEType(extension string) string {
	switch extension {
	case ".jpg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	default:
		return "image/webp"
	}
}

func webpDimensions(data []byte) (int, int, bool) {
	if len(data) < 30 || string(data[:4]) != "RIFF" || string(data[8:12]) != "WEBP" {
		return 0, 0, false
	}
	switch string(data[12:16]) {
	case "VP8X":
		width := 1 + int(data[24]) + int(data[25])<<8 + int(data[26])<<16
		height := 1 + int(data[27]) + int(data[28])<<8 + int(data[29])<<16
		return width, height, true
	case "VP8 ":
		if data[23] != 0x9d || data[24] != 0x01 || data[25] != 0x2a {
			return 0, 0, false
		}
		width := int(data[26]) | int(data[27]&0x3f)<<8
		height := int(data[28]) | int(data[29]&0x3f)<<8
		return width, height, width > 0 && height > 0
	case "VP8L":
		if data[20] != 0x2f {
			return 0, 0, false
		}
		bits := uint32(data[21]) | uint32(data[22])<<8 | uint32(data[23])<<16 | uint32(data[24])<<24
		width := int(bits&0x3fff) + 1
		height := int((bits>>14)&0x3fff) + 1
		return width, height, true
	default:
		return 0, 0, false
	}
}
