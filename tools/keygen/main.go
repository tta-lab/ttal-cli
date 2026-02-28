// ttal-keygen generates Ed25519 keypairs and signs license JWTs for TTAL.
//
// Usage:
//
//	ttal-keygen generate                          # generate keypair
//	ttal-keygen sign <private-key-b64> <email> <tier>  # sign a license JWT
package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "generate":
		cmdGenerate()
	case "sign":
		if len(os.Args) != 5 {
			fmt.Fprintf(os.Stderr, "usage: ttal-keygen sign <private-key-b64> <email> <tier>\n")
			os.Exit(1)
		}
		cmdSign(os.Args[2], os.Args[3], os.Args[4])
	default:
		usage()
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: ttal-keygen <generate|sign>")
	fmt.Fprintln(os.Stderr, "  generate                              generate Ed25519 keypair")
	fmt.Fprintln(os.Stderr, "  sign <private-key-b64> <email> <tier>  sign a license JWT (tier: pro|team)")
}

func cmdGenerate() {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	pubB64 := base64.StdEncoding.EncodeToString(pub)
	privB64 := base64.StdEncoding.EncodeToString(priv)

	fmt.Printf("PUBLIC_KEY=%s\n", pubB64)
	fmt.Printf("PRIVATE_KEY=%s\n", privB64)
	fmt.Fprintf(os.Stderr, "\nCopy PUBLIC_KEY into internal/license/pubkey.b64\n")
	fmt.Fprintf(os.Stderr, "Keep PRIVATE_KEY secret — use it with 'ttal-keygen sign'\n")
}

func cmdSign(privKeyB64, email, tier string) {
	if tier != "pro" && tier != "team" {
		fmt.Fprintf(os.Stderr, "error: tier must be 'pro' or 'team'\n")
		os.Exit(1)
	}

	privBytes, err := base64.StdEncoding.DecodeString(privKeyB64)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: decode private key: %v\n", err)
		os.Exit(1)
	}
	if len(privBytes) != ed25519.PrivateKeySize {
		fmt.Fprintf(os.Stderr, "error: invalid private key size: %d (expected %d)\n", len(privBytes), ed25519.PrivateKeySize)
		os.Exit(1)
	}
	priv := ed25519.PrivateKey(privBytes)

	// Build JWT: header.payload.signature
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"EdDSA","typ":"JWT"}`))

	payload := map[string]string{"sub": email, "tier": tier}
	payloadJSON, err2 := json.Marshal(payload)
	if err2 != nil {
		fmt.Fprintf(os.Stderr, "error: marshal payload: %v\n", err2)
		os.Exit(1)
	}
	payloadB64 := base64.RawURLEncoding.EncodeToString(payloadJSON)

	signingInput := header + "." + payloadB64
	sig := ed25519.Sign(priv, []byte(signingInput))
	sigB64 := base64.RawURLEncoding.EncodeToString(sig)

	token := strings.Join([]string{header, payloadB64, sigB64}, ".")
	fmt.Println(token)
}
