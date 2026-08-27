package main

import (
	"fmt"
	"os"

	"qq-pet-saas/updater"
)

func main() {
	if len(os.Args) != 3 {
		_, _ = fmt.Fprintln(os.Stderr, "usage: verifymanifest <manifest> <signature>")
		os.Exit(2)
	}
	raw, err := os.ReadFile(os.Args[1])
	if err != nil {
		fatal(err)
	}
	signature, err := os.ReadFile(os.Args[2])
	if err != nil {
		fatal(err)
	}
	manifest, err := updater.ParseAndVerifyManifest(raw, signature, updater.DefaultPublicKey)
	if err != nil {
		fatal(err)
	}
	_, _ = fmt.Fprintf(os.Stdout, "verified update manifest %s\n", manifest.Version)
}

func fatal(err error) {
	_, _ = fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
