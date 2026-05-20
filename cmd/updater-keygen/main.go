// updater-keygen generates an Ed25519 keypair for the software update
// signing pipeline. Run it once locally and store the printed values in
// the project's GitHub Secrets:
//
//   NETCATCHER_UPDATE_VERIFY_PUBLIC_KEY   = base64 of the 32-byte public key
//   NETCATCHER_UPDATE_SIGNING_PRIVATE_KEY = base64 of the 64-byte private key
//
// Never commit the private key.
package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"
)

func main() {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		fmt.Fprintf(os.Stderr, "generate: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("=== NetCatcher updater signing keypair ===")
	fmt.Println()
	fmt.Println("PUBLIC KEY (set as GitHub Secret NETCATCHER_UPDATE_VERIFY_PUBLIC_KEY):")
	fmt.Println(base64.StdEncoding.EncodeToString(pub))
	fmt.Println()
	fmt.Println("PRIVATE KEY (set as GitHub Secret NETCATCHER_UPDATE_SIGNING_PRIVATE_KEY):")
	fmt.Println(base64.StdEncoding.EncodeToString(priv))
	fmt.Println()
	fmt.Println("Store the PRIVATE KEY in a password manager. Do NOT commit it.")
}
