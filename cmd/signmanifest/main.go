package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"flag"
	"fmt"
	"os"
	"strings"
)

func main() {
	manifest := flag.String("manifest", "", "manifest file to sign")
	output := flag.String("output", "", "signature output file")
	generate := flag.Bool("generate", false, "generate a new Ed25519 key pair")
	publicOutput := flag.String("public-output", "", "generated public key file")
	privateOutput := flag.String("private-output", "", "generated private key file")
	flag.Parse()
	if *generate {
		if *publicOutput == "" || *privateOutput == "" {
			fatal("-generate requires -public-output and -private-output")
		}
		publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			fatal(err.Error())
		}
		if err := os.WriteFile(*publicOutput, []byte(base64.StdEncoding.EncodeToString(publicKey)+"\n"), 0o644); err != nil {
			fatal(err.Error())
		}
		if err := os.WriteFile(*privateOutput, []byte(base64.StdEncoding.EncodeToString(privateKey)+"\n"), 0o600); err != nil {
			fatal(err.Error())
		}
		return
	}
	if *manifest == "" || *output == "" {
		fatal("-manifest and -output are required")
	}
	privateKey, err := base64.StdEncoding.DecodeString(strings.TrimSpace(os.Getenv("UPDATE_SIGNING_PRIVATE_KEY")))
	if err != nil || len(privateKey) != ed25519.PrivateKeySize {
		fatal("UPDATE_SIGNING_PRIVATE_KEY must be a base64 encoded Ed25519 private key")
	}
	raw, err := os.ReadFile(*manifest)
	if err != nil {
		fatal(err.Error())
	}
	signature := ed25519.Sign(ed25519.PrivateKey(privateKey), raw)
	if err := os.WriteFile(*output, []byte(base64.StdEncoding.EncodeToString(signature)+"\n"), 0o644); err != nil {
		fatal(err.Error())
	}
}

func fatal(message string) {
	_, _ = fmt.Fprintln(os.Stderr, message)
	os.Exit(1)
}
