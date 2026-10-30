package siwx

import "time"

type Message struct {
	Raw             string
	Scheme          string
	Domain          string
	Network         string
	Address         string
	Statement       string
	SignedStatement string
	URI             string
	Version         string
	Namespace       string
	ChainID         string
	Nonce           string
	IssuedAt        time.Time
	ExpirationTime  *time.Time
	NotBefore       *time.Time
	RequestID       string
	Resources       []string
}
