// sign-checksums signs SHA256SUMS with the Ed25519 private key supplied
// in the env var UPDATE_SIGNING_PRIVATE_KEY (base64 of the 64-byte raw
// key). It writes a single line containing base64(signature) to the
// output path.
//
// Usage: go run .github/scripts/sign-checksums.go <sums-file> <sig-out>
package main

import (
	"crypto/ed25519"
	"encoding/base64"
	"fmt"
	"os"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: sign-checksums <sums-file> <sig-out>")
		os.Exit(2)
	}
	keyB64 := os.Getenv("UPDATE_SIGNING_PRIVATE_KEY")
	if keyB64 == "" {
		fmt.Fprintln(os.Stderr, "UPDATE_SIGNING_PRIVATE_KEY env var is required")
		os.Exit(2)
	}
	rawKey, err := base64.StdEncoding.DecodeString(keyB64)
	if err != nil {
		fmt.Fprintf(os.Stderr, "decode key: %v\n", err)
		os.Exit(1)
	}
	if len(rawKey) != ed25519.PrivateKeySize {
		fmt.Fprintf(os.Stderr, "private key size %d, want %d\n", len(rawKey), ed25519.PrivateKeySize)
		os.Exit(1)
	}

	sums, err := os.ReadFile(os.Args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "read sums: %v\n", err)
		os.Exit(1)
	}
	sig := ed25519.Sign(ed25519.PrivateKey(rawKey), sums)
	out := base64.StdEncoding.EncodeToString(sig) + "\n"
	if err := os.WriteFile(os.Args[2], []byte(out), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "write sig: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("signed %s (%d bytes) -> %s\n", os.Args[1], len(sums), os.Args[2])
}
