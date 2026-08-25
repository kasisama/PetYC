package qqofficial

import (
	"bytes"
	"context"
	"crypto/md5"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"qq-pet-saas/config"
	"qq-pet-saas/core"
)

const qqImageSoftLimit = 20 * 1024 * 1024
const qqMD5PrefixBytes = 10_002_432

type mediaUploadResult struct {
	FileInfo string `json:"file_info"`
}

type uploadPrepareResult struct {
	UploadID  string       `json:"upload_id"`
	BlockSize string       `json:"block_size"`
	Parts     []uploadPart `json:"parts"`
}

type uploadPart struct {
	Index        int    `json:"index"`
	PresignedURL string `json:"presigned_url"`
	BlockSize    string `json:"block_size"`
}

func (client *Client) sendGroupMedia(ctx context.Context, event core.InboundEvent, fileInfo, token string) (*SendResult, error) {
	payload := map[string]interface{}{"msg_type": 7, "media": map[string]string{"file_info": fileInfo}}
	if event.MessageID != "" {
		payload["msg_id"] = event.MessageID
		payload["msg_seq"] = 1
		payload["message_reference"] = map[string]interface{}{"message_id": event.MessageID, "ignore_get_message_error": true}
	}
	var result SendResult
	endpoint := fmt.Sprintf("%s/v2/groups/%s/messages", client.BaseURL, url.PathEscape(event.SpaceID))
	if err := client.authorizedJSON(ctx, event, http.MethodPost, endpoint, token, payload, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (client *Client) uploadGroupImage(ctx context.Context, event core.InboundEvent, source, token string) (string, error) {
	source = strings.TrimSpace(source)
	if source == "" {
		return "", fmt.Errorf("图片路径为空")
	}
	if isHTTPURL(source) {
		return client.uploadGroupImageURL(ctx, event, source, token)
	}
	data, fileName, err := readConfiguredImage(source)
	if err != nil {
		if publicURL := configuredImageURL(source); publicURL != "" {
			return client.uploadGroupImageURL(ctx, event, publicURL, token)
		}
		return "", err
	}
	if len(data) == 0 || len(data) > qqImageSoftLimit {
		return "", fmt.Errorf("图片大小必须在 1B 至 20MB 之间")
	}
	return client.uploadGroupImageData(ctx, event, fileName, data, token)
}

func (client *Client) uploadGroupImageURL(ctx context.Context, event core.InboundEvent, source, token string) (string, error) {
	payload := map[string]interface{}{"file_type": 1, "url": source, "srv_send_msg": false}
	var result mediaUploadResult
	endpoint := fmt.Sprintf("%s/v2/groups/%s/files", client.BaseURL, url.PathEscape(event.SpaceID))
	if err := client.authorizedJSON(ctx, event, http.MethodPost, endpoint, token, payload, &result); err != nil {
		return "", err
	}
	if result.FileInfo == "" {
		return "", fmt.Errorf("QQ 图片上传响应缺少 file_info")
	}
	return result.FileInfo, nil
}

func (client *Client) uploadGroupImageData(ctx context.Context, event core.InboundEvent, fileName string, data []byte, token string) (string, error) {
	wholeMD5 := md5.Sum(data)
	wholeSHA1 := sha1.Sum(data)
	prefixEnd := len(data)
	if prefixEnd > qqMD5PrefixBytes {
		prefixEnd = qqMD5PrefixBytes
	}
	prefixMD5 := md5.Sum(data[:prefixEnd])
	payload := map[string]interface{}{
		"file_type": 1,
		"file_size": strconv.Itoa(len(data)),
		"file_name": fileName,
		"md5":       hex.EncodeToString(wholeMD5[:]),
		"sha1":      hex.EncodeToString(wholeSHA1[:]),
		"md5_10m":   hex.EncodeToString(prefixMD5[:]),
	}
	base := fmt.Sprintf("%s/v2/groups/%s", client.BaseURL, url.PathEscape(event.SpaceID))
	var prepared uploadPrepareResult
	if err := client.authorizedJSON(ctx, event, http.MethodPost, base+"/upload_prepare", token, payload, &prepared); err != nil {
		return "", err
	}
	if prepared.UploadID == "" {
		return "", fmt.Errorf("QQ 图片预上传响应缺少 upload_id")
	}
	blockSize, err := strconv.Atoi(prepared.BlockSize)
	if err != nil || blockSize <= 0 {
		return "", fmt.Errorf("QQ 图片预上传返回无效 block_size")
	}
	sort.Slice(prepared.Parts, func(i, j int) bool { return prepared.Parts[i].Index < prepared.Parts[j].Index })
	uploadedBytes := 0
	for _, part := range prepared.Parts {
		if uploadedBytes >= len(data) || part.PresignedURL == "" {
			return "", fmt.Errorf("QQ 图片预上传返回无效分片 %d", part.Index)
		}
		partSize := blockSize
		if value, parseErr := strconv.Atoi(part.BlockSize); parseErr == nil && value > 0 {
			partSize = value
		}
		end := uploadedBytes + partSize
		if end > len(data) {
			end = len(data)
		}
		chunk := data[uploadedBytes:end]
		if err = client.putPresignedPart(ctx, event, part.PresignedURL, chunk); err != nil {
			return "", fmt.Errorf("上传 QQ 图片分片 %d: %w", part.Index, err)
		}
		chunkMD5 := md5.Sum(chunk)
		finish := map[string]interface{}{
			"upload_id":  prepared.UploadID,
			"part_index": part.Index,
			"block_size": strconv.Itoa(len(chunk)),
			"md5":        hex.EncodeToString(chunkMD5[:]),
		}
		if err = client.authorizedJSON(ctx, event, http.MethodPost, base+"/upload_part_finish", token, finish, nil); err != nil {
			return "", fmt.Errorf("确认 QQ 图片分片 %d: %w", part.Index, err)
		}
		uploadedBytes = end
	}
	if uploadedBytes != len(data) {
		return "", fmt.Errorf("QQ 图片预上传分片不完整: 已覆盖 %d/%d 字节", uploadedBytes, len(data))
	}
	merge := map[string]interface{}{"file_type": 1, "srv_send_msg": false, "file_name": fileName, "upload_id": prepared.UploadID}
	var result mediaUploadResult
	if err = client.authorizedJSON(ctx, event, http.MethodPost, base+"/files", token, merge, &result); err != nil {
		return "", err
	}
	if result.FileInfo == "" {
		return "", fmt.Errorf("QQ 图片合并响应缺少 file_info")
	}
	return result.FileInfo, nil
}

func (client *Client) putPresignedPart(ctx context.Context, event core.InboundEvent, endpoint string, data []byte) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodPut, endpoint, bytes.NewReader(data))
	if err != nil {
		return err
	}
	request.ContentLength = int64(len(data))
	if err = client.waitForRequest(ctx, event); err != nil {
		return err
	}
	response, err := client.HTTPClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("HTTP %d", response.StatusCode)
	}
	return nil
}

func (client *Client) authorizedJSON(ctx context.Context, event core.InboundEvent, method, endpoint, token string, payload interface{}, target interface{}) error {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, method, endpoint, bytes.NewReader(encoded))
	if err != nil {
		return err
	}
	request.Header.Set("Authorization", "QQBot "+token)
	request.Header.Set("Content-Type", "application/json")
	if err = client.waitForRequest(ctx, event); err != nil {
		return err
	}
	response, err := client.HTTPClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		return fmt.Errorf("QQ 媒体接口失败: HTTP %d %s", response.StatusCode, strings.TrimSpace(string(body)))
	}
	if target == nil || response.StatusCode == http.StatusNoContent {
		return nil
	}
	if err = json.NewDecoder(response.Body).Decode(target); err != nil && err != io.EOF {
		return err
	}
	return nil
}

func readConfiguredImage(source string) ([]byte, string, error) {
	normalized := strings.TrimPrefix(strings.ReplaceAll(strings.TrimSpace(source), "\\", "/"), "./")
	normalized = strings.TrimPrefix(normalized, "图片/")
	clean := filepath.Clean(filepath.FromSlash(normalized))
	if clean == "." || filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return nil, "", fmt.Errorf("非法图片路径")
	}
	roots := []string{filepath.Join(config.GlobalConfigPath, "图片"), filepath.Join(".", "图片")}
	var lastErr error
	for _, root := range roots {
		absoluteRoot, err := filepath.Abs(root)
		if err != nil {
			lastErr = err
			continue
		}
		candidate, err := filepath.Abs(filepath.Join(absoluteRoot, clean))
		if err != nil {
			lastErr = err
			continue
		}
		relative, err := filepath.Rel(absoluteRoot, candidate)
		if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			lastErr = fmt.Errorf("图片路径越界")
			continue
		}
		data, err := os.ReadFile(candidate)
		if err == nil {
			return data, filepath.Base(candidate), nil
		}
		lastErr = err
	}
	return nil, "", fmt.Errorf("读取图片 %q 失败: %w", source, lastErr)
}

func configuredImageURL(source string) string {
	host := strings.TrimRight(strings.TrimSpace(config.Core.ImageHost), "/")
	if host == "" || !isHTTPURL(host) {
		return ""
	}
	normalized := strings.TrimPrefix(strings.ReplaceAll(strings.TrimSpace(source), "\\", "/"), "./")
	normalized = strings.TrimPrefix(normalized, "图片/")
	segments := strings.Split(normalized, "/")
	for index, segment := range segments {
		segments[index] = url.PathEscape(segment)
	}
	return host + "/images/" + strings.Join(segments, "/")
}

func isHTTPURL(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Host != ""
}
