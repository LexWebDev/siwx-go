package siwx

import (
	"context"
	"errors"
	"net/url"
	"slices"
	"strings"
	"time"
)

const defaultClockSkew = 5 * time.Minute

type SignatureVerifier interface {
	Namespace() string
	Verify(ctx context.Context, message *Message, signature string) bool
	NormaliseAddress(address string) string
}

type ContractSignatureChecker interface {
	IsValid(ctx context.Context, address, hash, signature, chainID string) bool
}

type NonceStore interface {
	Issue(ctx context.Context) (string, error)
	Exists(ctx context.Context, nonce string) (bool, error)
	Consume(ctx context.Context, nonce string) error
}

type Config struct {
	Domains    []string
	Namespaces []string
	ClockSkew  time.Duration
	Nonces     NonceStore
	Contracts  ContractSignatureChecker
	Verifiers  []SignatureVerifier
	Now        func() time.Time
}

type Session struct {
	Address   string
	Namespace string
	ChainID   string
	Domain    string
	IssuedAt  time.Time
	Message   *Message
}

type Verifier struct {
	domains   []string
	enabled   []string
	verifiers map[string]SignatureVerifier
	nonces    NonceStore
	clockSkew time.Duration
	now       func() time.Time
}

func New(config Config) (*Verifier, error) {
	if config.Nonces == nil {
		return nil, errors.New("siwx: a nonce store is required")
	}

	verifier := &Verifier{
		domains:   normaliseDomains(config.Domains),
		enabled:   config.Namespaces,
		verifiers: map[string]SignatureVerifier{},
		nonces:    config.Nonces,
		clockSkew: config.ClockSkew,
		now:       config.Now,
	}

	if verifier.enabled == nil {
		verifier.enabled = []string{"eip155", "solana"}
	}
	if verifier.clockSkew == 0 {
		verifier.clockSkew = defaultClockSkew
	}
	if verifier.now == nil {
		verifier.now = time.Now
	}

	builtIn := []SignatureVerifier{NewEip155Verifier(config.Contracts), NewSolanaVerifier()}

	for _, registered := range append(builtIn, config.Verifiers...) {
		verifier.verifiers[registered.Namespace()] = registered
	}

	return verifier, nil
}

func (verifier *Verifier) Verify(ctx context.Context, raw, signature string) (*Session, error) {
	message, err := Parse(raw)
	if err != nil {
		return nil, err
	}

	chainVerifier, err := verifier.verifierFor(message.Namespace)
	if err != nil {
		return nil, err
	}

	if err := verifier.assertDomain(message); err != nil {
		return nil, err
	}

	if message.Version != "1" {
		return nil, ErrInvalidMessage
	}

	if err := verifier.assertTimestamps(message); err != nil {
		return nil, err
	}

	live, err := verifier.nonces.Exists(ctx, message.Nonce)
	if err != nil {
		return nil, err
	}
	if !live {
		return nil, ErrInvalidNonce
	}

	if !chainVerifier.Verify(ctx, message, signature) {
		return nil, ErrInvalidSignature
	}

	if err := verifier.nonces.Consume(ctx, message.Nonce); err != nil {
		return nil, err
	}

	return &Session{
		Address:   chainVerifier.NormaliseAddress(message.Address),
		Namespace: message.Namespace,
		ChainID:   message.ChainID,
		Domain:    message.Domain,
		IssuedAt:  message.IssuedAt,
		Message:   message,
	}, nil
}

func (verifier *Verifier) verifierFor(namespace string) (SignatureVerifier, error) {
	if !slices.Contains(verifier.enabled, namespace) {
		return nil, ErrUnsupportedNamespace
	}

	chainVerifier, registered := verifier.verifiers[namespace]
	if !registered {
		return nil, ErrUnsupportedNamespace
	}

	return chainVerifier, nil
}

func (verifier *Verifier) assertDomain(message *Message) error {
	if !slices.Contains(verifier.domains, message.Domain) {
		return ErrInvalidDomain
	}

	parsed, err := url.Parse(message.URI)
	if err != nil || parsed.Host != message.Domain {
		return ErrInvalidDomain
	}

	return nil
}

func (verifier *Verifier) assertTimestamps(message *Message) error {
	now := verifier.now()

	if message.IssuedAt.After(now.Add(verifier.clockSkew)) {
		return ErrInvalidMessage
	}

	if message.ExpirationTime != nil && message.ExpirationTime.Before(now) {
		return ErrInvalidMessage
	}

	if message.NotBefore != nil && message.NotBefore.After(now) {
		return ErrInvalidMessage
	}

	return nil
}

func normaliseDomains(domains []string) []string {
	normalised := make([]string, 0, len(domains))

	for _, domain := range domains {
		if trimmed := strings.TrimSpace(domain); trimmed != "" {
			normalised = append(normalised, trimmed)
		}
	}

	return normalised
}
