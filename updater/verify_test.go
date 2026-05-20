package updater

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"testing"
)

func makeKey(t *testing.T) (pubB64, privB64 string) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("genkey: %v", err)
	}
	return base64.StdEncoding.EncodeToString(pub), base64.StdEncoding.EncodeToString(priv)
}

func sign(t *testing.T, privB64 string, data []byte) string {
	t.Helper()
	raw, err := base64.StdEncoding.DecodeString(privB64)
	if err != nil {
		t.Fatalf("decode priv: %v", err)
	}
	return base64.StdEncoding.EncodeToString(ed25519.Sign(ed25519.PrivateKey(raw), data))
}

func sha(b []byte) []byte { h := sha256.Sum256(b); return h[:] }
func hexs(b []byte) string { return hex.EncodeToString(b) }

func TestVerifyChecksumsHappy(t *testing.T) {
	asset := []byte("hello world")
	sums := []byte(fmt.Sprintf("%s  NetCatcher-arm64.app.tar.gz\n", hexs(sha(asset))))
	pub, priv := makeKey(t)
	sig := sign(t, priv, sums)

	if err := VerifyChecksums(pub, sums, mustB64(sig), "NetCatcher-arm64.app.tar.gz", sha(asset)); err != nil {
		t.Fatalf("expected pass, got %v", err)
	}
}

func TestVerifyChecksumsTamperedSums(t *testing.T) {
	asset := []byte("hello world")
	sums := []byte(fmt.Sprintf("%s  NetCatcher-arm64.app.tar.gz\n", hexs(sha(asset))))
	pub, priv := makeKey(t)
	sig := sign(t, priv, sums)
	// Tamper.
	sums[0] ^= 0x01

	err := VerifyChecksums(pub, sums, mustB64(sig), "NetCatcher-arm64.app.tar.gz", sha(asset))
	if err == nil {
		t.Fatalf("expected signature failure on tampered sums")
	}
}

func TestVerifyChecksumsBadSig(t *testing.T) {
	asset := []byte("hello world")
	sums := []byte(fmt.Sprintf("%s  NetCatcher-arm64.app.tar.gz\n", hexs(sha(asset))))
	pub, _ := makeKey(t)
	_, otherPriv := makeKey(t)
	sig := sign(t, otherPriv, sums)

	if err := VerifyChecksums(pub, sums, mustB64(sig), "NetCatcher-arm64.app.tar.gz", sha(asset)); err == nil {
		t.Fatalf("expected signature failure with mismatched key")
	}
}

func TestVerifyChecksumsAssetMismatch(t *testing.T) {
	asset := []byte("hello world")
	sums := []byte(fmt.Sprintf("%s  NetCatcher-arm64.app.tar.gz\n", hexs(sha(asset))))
	pub, priv := makeKey(t)
	sig := sign(t, priv, sums)

	if err := VerifyChecksums(pub, sums, mustB64(sig), "NetCatcher-arm64.app.tar.gz", sha([]byte("different"))); err == nil {
		t.Fatalf("expected SHA mismatch error")
	}
}

func TestVerifyChecksumsAssetMissing(t *testing.T) {
	asset := []byte("hello world")
	sums := []byte(fmt.Sprintf("%s  Other-Asset.zip\n", hexs(sha(asset))))
	pub, priv := makeKey(t)
	sig := sign(t, priv, sums)

	if err := VerifyChecksums(pub, sums, mustB64(sig), "NetCatcher-arm64.app.tar.gz", sha(asset)); err == nil {
		t.Fatalf("expected missing-asset error")
	}
}

func TestVerifyChecksumsEmptyPubKeySkipsEd25519(t *testing.T) {
	asset := []byte("hello world")
	sums := []byte(fmt.Sprintf("%s  NetCatcher-arm64.app.tar.gz\n", hexs(sha(asset))))

	// Empty pub key, any sig — must pass as long as SHA matches.
	if err := VerifyChecksums("", sums, nil, "NetCatcher-arm64.app.tar.gz", sha(asset)); err != nil {
		t.Fatalf("expected pass with empty pub key, got %v", err)
	}
}

func TestVerifyChecksumsEmptyPubKeyStillEnforcesSHA(t *testing.T) {
	asset := []byte("hello world")
	sums := []byte(fmt.Sprintf("%s  NetCatcher-arm64.app.tar.gz\n", hexs(sha(asset))))

	if err := VerifyChecksums("", sums, nil, "NetCatcher-arm64.app.tar.gz", sha([]byte("nope"))); err == nil {
		t.Fatalf("expected SHA mismatch even with empty pub key")
	}
}

func mustB64(s string) []byte {
	raw, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		panic(err)
	}
	return raw
}
