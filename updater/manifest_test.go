package updater

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
)

func signedManifest(t *testing.T, manifest Manifest) ([]byte, []byte, string) {
	t.Helper()
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	signature := ed25519.Sign(privateKey, raw)
	return raw, []byte(base64.StdEncoding.EncodeToString(signature)), base64.StdEncoding.EncodeToString(publicKey)
}

func validManifest() Manifest {
	return Manifest{
		Schema: 1, Version: "1.2.3", Channel: "stable", PublishedAt: "2026-08-27T00:00:00Z", Notes: "test",
		Platforms: map[string]Artifact{
			"windows-amd64": {URL: "https://example.com/petyc.exe", SHA256: strings.Repeat("a", 64), Size: 42},
		},
	}
}

func TestParseAndVerifyManifest(t *testing.T) {
	raw, signature, publicKey := signedManifest(t, validManifest())
	manifest, err := ParseAndVerifyManifest(raw, signature, publicKey)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.Version != "1.2.3" {
		t.Fatalf("unexpected version %q", manifest.Version)
	}

	raw[len(raw)-1] ^= 1
	if _, err := ParseAndVerifyManifest(raw, signature, publicKey); err == nil {
		t.Fatal("tampered manifest should fail signature verification")
	}
}

func TestArtifactAcceptsOptionalHTTPSMirrors(t *testing.T) {
	artifact := Artifact{
		URL:     "https://downloads.example.com/petyc.exe",
		Mirrors: []string{"https://github.com/example/petyc.exe"},
		SHA256:  strings.Repeat("a", 64), Size: 42,
	}
	if err := artifact.Validate(); err != nil {
		t.Fatal(err)
	}
	artifact.Mirrors = append(artifact.Mirrors, "http://unsafe.example/petyc.exe")
	if err := artifact.Validate(); err == nil {
		t.Fatal("non-HTTPS mirror should fail validation")
	}
}

func TestArtifactWithoutMirrorsRemainsCompatible(t *testing.T) {
	var artifact Artifact
	if err := json.Unmarshal([]byte(`{"url":"https://example.com/petyc.exe","sha256":"`+strings.Repeat("a", 64)+`","size":42}`), &artifact); err != nil {
		t.Fatal(err)
	}
	if err := artifact.Validate(); err != nil {
		t.Fatal(err)
	}
	addresses := artifact.downloadURLs()
	if len(addresses) != 1 || addresses[0] != artifact.URL {
		t.Fatalf("unexpected download URLs: %#v", addresses)
	}
}

func TestNewerVersion(t *testing.T) {
	tests := []struct {
		candidate, current string
		expected           bool
	}{
		{"1.2.4", "1.2.3", true}, {"1.3.0", "1.2.9", true}, {"2.0.0", "1.9.9", true},
		{"1.2.3", "1.2.3", false}, {"1.2.2", "1.2.3", false},
	}
	for _, test := range tests {
		actual, err := newerVersion(test.candidate, test.current)
		if err != nil {
			t.Fatalf("%s/%s: %v", test.candidate, test.current, err)
		}
		if actual != test.expected {
			t.Fatalf("%s > %s = %v, want %v", test.candidate, test.current, actual, test.expected)
		}
	}
	if _, err := newerVersion("nightly", "1.0.0"); err == nil {
		t.Fatal("invalid version should fail")
	}
}
