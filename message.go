package siwx

import "time"

// Message is a parsed EIP-4361 message.
type Message struct {
	// Raw is the normalised text the signature covers.
	Raw     string
	Scheme  string
	Domain  string
	Network string
	Address string
	// Statement is what the app asked to sign, without the ERC-5573 recap sentence.
	Statement string
	// SignedStatement is the statement line exactly as it was signed.
	SignedStatement string
	URI             string
	Version         string
	Namespace       string
	// ChainID is always in CAIP-2 form, such as eip155:1.
	ChainID        string
	Nonce          string
	IssuedAt       time.Time
	ExpirationTime *time.Time
	NotBefore      *time.Time
	RequestID      string
	Resources      []string
}
