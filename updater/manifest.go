package updater

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

const manifestSchema = 1

var strictVersionPattern = regexp.MustCompile(`^(?:v)?(\d+)\.(\d+)\.(\d+)$`)

type Artifact struct {
	URL     string   `json:"url"`
	Mirrors []string `json:"mirrors,omitempty"`
	SHA256  string   `json:"sha256"`
	Size    int64    `json:"size"`
}

type Manifest struct {
	Schema      int                 `json:"schema"`
	Version     string              `json:"version"`
	Channel     string              `json:"channel"`
	PublishedAt string              `json:"publishedAt"`
	Notes       string              `json:"notes"`
	Platforms   map[string]Artifact `json:"platforms"`
}

func ParseAndVerifyManifest(raw, encodedSignature []byte, encodedPublicKey string) (Manifest, error) {
	publicKey, err := base64.StdEncoding.DecodeString(strings.TrimSpace(encodedPublicKey))
	if err != nil || len(publicKey) != ed25519.PublicKeySize {
		return Manifest{}, errors.New("更新公钥配置无效")
	}
	signature, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(encodedSignature)))
	if err != nil || len(signature) != ed25519.SignatureSize {
		return Manifest{}, errors.New("更新清单签名格式无效")
	}
	if !ed25519.Verify(ed25519.PublicKey(publicKey), raw, signature) {
		return Manifest{}, errors.New("更新清单签名验证失败")
	}

	var manifest Manifest
	if err := json.Unmarshal(raw, &manifest); err != nil {
		return Manifest{}, fmt.Errorf("解析更新清单: %w", err)
	}
	if err := manifest.Validate(); err != nil {
		return Manifest{}, err
	}
	return manifest, nil
}

func (manifest Manifest) Validate() error {
	if manifest.Schema != manifestSchema {
		return fmt.Errorf("不支持的更新清单版本: %d", manifest.Schema)
	}
	if _, err := parseVersion(manifest.Version); err != nil {
		return fmt.Errorf("更新版本无效: %w", err)
	}
	if manifest.Channel != "stable" {
		return fmt.Errorf("不支持的更新通道: %s", manifest.Channel)
	}
	if len(manifest.Platforms) == 0 {
		return errors.New("更新清单没有平台产物")
	}
	for platform, artifact := range manifest.Platforms {
		if err := artifact.Validate(); err != nil {
			return fmt.Errorf("平台 %s: %w", platform, err)
		}
	}
	return nil
}

func (artifact Artifact) Validate() error {
	addresses := append([]string{artifact.URL}, artifact.Mirrors...)
	for _, address := range addresses {
		parsed, err := url.Parse(address)
		if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
			return errors.New("下载地址必须是有效 HTTPS URL")
		}
	}
	checksum, err := hex.DecodeString(strings.TrimSpace(artifact.SHA256))
	if err != nil || len(checksum) != sha256.Size {
		return errors.New("SHA-256 无效")
	}
	if artifact.Size <= 0 {
		return errors.New("文件大小必须大于 0")
	}
	return nil
}

func (artifact Artifact) downloadURLs() []string {
	addresses := make([]string, 0, 1+len(artifact.Mirrors))
	seen := make(map[string]struct{}, 1+len(artifact.Mirrors))
	for _, address := range append([]string{artifact.URL}, artifact.Mirrors...) {
		address = strings.TrimSpace(address)
		if address == "" {
			continue
		}
		if _, exists := seen[address]; exists {
			continue
		}
		seen[address] = struct{}{}
		addresses = append(addresses, address)
	}
	return addresses
}

type semanticVersion [3]int

func parseVersion(value string) (semanticVersion, error) {
	matches := strictVersionPattern.FindStringSubmatch(strings.TrimSpace(value))
	if len(matches) != 4 {
		return semanticVersion{}, fmt.Errorf("必须是 x.y.z 格式")
	}
	var result semanticVersion
	for index := range result {
		part, err := strconv.Atoi(matches[index+1])
		if err != nil {
			return semanticVersion{}, err
		}
		result[index] = part
	}
	return result, nil
}

func newerVersion(candidate, current string) (bool, error) {
	left, err := parseVersion(candidate)
	if err != nil {
		return false, err
	}
	right, err := parseVersion(current)
	if err != nil {
		return false, err
	}
	for index := range left {
		if left[index] != right[index] {
			return left[index] > right[index], nil
		}
	}
	return false, nil
}
