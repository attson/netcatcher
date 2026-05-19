package updater

import (
	"bufio"
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
)

// ErrSignatureFailed is returned when Ed25519 verification of SHA256SUMS
// does not match. The download must be kept on disk for forensics.
var ErrSignatureFailed = errors.New("signature verification failed")

// ErrChecksumMismatch is returned when the asset's SHA256 does not match
// the line in SHA256SUMS. The download is safe to delete and retry.
var ErrChecksumMismatch = errors.New("checksum mismatch")

// ErrAssetNotInSums is returned when SHA256SUMS contains no line for the
// requested asset name.
var ErrAssetNotInSums = errors.New("asset not present in SHA256SUMS")

// VerifyChecksums returns nil iff:
//   - publicKeyB64 == "" OR ed25519.Verify(pub, sumsBytes, sigBytes) == true
//   - the line in sumsBytes for assetName has SHA256 hex matching assetSHA256
//
// sigBytes must be the raw 64-byte signature (not base64). Callers decode it.
// publicKeyB64 is base64 of the 32-byte public key.
func VerifyChecksums(publicKeyB64 string, sumsBytes, sigBytes []byte, assetName string, assetSHA256 []byte) error {
	if publicKeyB64 != "" {
		pub, err := base64.StdEncoding.DecodeString(publicKeyB64)
		if err != nil {
			return fmt.Errorf("decode public key: %w", err)
		}
		if len(pub) != ed25519.PublicKeySize {
			return fmt.Errorf("public key size %d, want %d", len(pub), ed25519.PublicKeySize)
		}
		if !ed25519.Verify(ed25519.PublicKey(pub), sumsBytes, sigBytes) {
			return ErrSignatureFailed
		}
	}

	wantHex, ok := findChecksum(sumsBytes, assetName)
	if !ok {
		return fmt.Errorf("%w: %s", ErrAssetNotInSums, assetName)
	}
	gotHex := hex.EncodeToString(assetSHA256)
	if !strings.EqualFold(wantHex, gotHex) {
		return fmt.Errorf("%w: want %s, got %s", ErrChecksumMismatch, wantHex, gotHex)
	}
	return nil
}

// findChecksum scans a `sha256sum`-style file. Each non-empty line is
// "<hex>  <filename>" or "<hex> *<filename>" (binary marker). Path
// separators are stripped — only the basename is compared.
func findChecksum(sumsBytes []byte, assetName string) (string, bool) {
	scanner := bufio.NewScanner(bytes.NewReader(sumsBytes))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		hash := fields[0]
		name := strings.TrimPrefix(fields[len(fields)-1], "*")
		// Normalise to basename.
		if idx := strings.LastIndexAny(name, "/\\"); idx >= 0 {
			name = name[idx+1:]
		}
		if name == assetName {
			return hash, true
		}
	}
	return "", false
}
