package siwx_test

import (
	"context"
	"crypto/ed25519"
	"errors"
	"fmt"
	"time"

	"github.com/mr-tron/base58"

	siwx "github.com/LexWebDev/siwx-go"
	"github.com/LexWebDev/siwx-go/evm"
	"github.com/LexWebDev/siwx-go/memorynonce"
	"github.com/LexWebDev/siwx-go/solana"
)

func ExampleVerifier_Verify() {
	store := memorynonce.New(10 * time.Minute)

	verifier, err := siwx.New(siwx.Config{
		Domains:   []string{"app.example"},
		Nonces:    store,
		Verifiers: []siwx.SignatureVerifier{evm.New(nil), solana.New()},
	})
	if err != nil {
		panic(err)
	}

	publicKey, privateKey, _ := ed25519.GenerateKey(nil)
	nonce, _ := store.Issue(context.Background())

	message := "app.example wants you to sign in with your Solana account:\n" +
		base58.Encode(publicKey) + "\n\nSign in.\n\n" +
		"URI: https://app.example\nVersion: 1\nChain ID: solana:mainnet\n" +
		"Nonce: " + nonce + "\nIssued At: " + time.Now().UTC().Format(time.RFC3339)

	signature := base58.Encode(ed25519.Sign(privateKey, []byte(message)))

	session, err := verifier.Verify(context.Background(), message, signature)
	if err != nil {
		panic(err)
	}

	fmt.Println(session.Namespace, session.ChainID)

	_, err = verifier.Verify(context.Background(), message, signature)
	fmt.Println(errors.Is(err, siwx.ErrInvalidNonce))

	// Output:
	// solana solana:mainnet
	// true
}

func ExampleParse() {
	message, err := siwx.Parse("app.example wants you to sign in with your Ethereum account:\n" +
		"0x70997970C51812dc3A010C7d01b50e0d17dc79C8\n\nSign in.\n\n" +
		"URI: https://app.example\nVersion: 1\nChain ID: 137\n" +
		"Nonce: 8Kc2fQ1pXm9Zr4Lt\nIssued At: 2026-07-30T12:34:56Z")
	if err != nil {
		panic(err)
	}

	fmt.Println(message.Domain, message.ChainID, message.Statement)

	// Output:
	// app.example eip155:137 Sign in.
}

func ExampleError() {
	_, err := siwx.Parse("not a sign-in message")

	var failure *siwx.Error
	if errors.As(err, &failure) {
		fmt.Println(failure.Code)
	}

	// Output:
	// siwx_invalid_message
}
