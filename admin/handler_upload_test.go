package admin

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"qq-pet-saas/database"
	"qq-pet-saas/models"
)

func TestUploadImageReturnsStandardEnvelopeAndPersistsRealPNG(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err = db.AutoMigrate(&models.ImageAsset{}); err != nil {
		t.Fatal(err)
	}
	previousDB := database.DB
	database.DB = db
	t.Cleanup(func() { database.DB = previousDB })

	previousDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	tempDir := t.TempDir()
	if err = os.Chdir(tempDir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(previousDir) })

	var imageBytes bytes.Buffer
	sample := image.NewRGBA(image.Rect(0, 0, 2, 3))
	sample.Set(0, 0, color.RGBA{R: 255, A: 255})
	if err = png.Encode(&imageBytes, sample); err != nil {
		t.Fatal(err)
	}
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "pet-sample.png")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = part.Write(imageBytes.Bytes()); err != nil {
		t.Fatal(err)
	}
	if err = writer.Close(); err != nil {
		t.Fatal(err)
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/api/admin/upload", UploadImage)
	request := httptest.NewRequest(http.MethodPost, "/api/admin/upload", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("上传返回 HTTP %d: %s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		Code int `json:"code"`
		Data struct {
			Path  string            `json:"path"`
			URL   string            `json:"url"`
			Asset models.ImageAsset `json:"asset"`
		} `json:"data"`
		RequestID string `json:"request_id"`
	}
	if err = json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Code != 0 || response.RequestID == "" || response.Data.Asset.Width != 2 || response.Data.Asset.Height != 3 {
		t.Fatalf("上传响应契约或图片元数据错误: %#v", response)
	}
	if _, err = os.Stat(filepath.Join("图片", response.Data.Path)); err != nil {
		t.Fatalf("上传文件未落入单一图片目录: %v", err)
	}
	var count int64
	if err = db.Model(&models.ImageAsset{}).Where("id = ?", response.Data.Asset.ID).Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("图片资产未登记: count=%d err=%v", count, err)
	}
}
