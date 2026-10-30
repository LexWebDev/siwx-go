package siwx

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"slices"
	"strings"
	"time"
)

const defaultClockSkew = 5 * time.Minute

// NoClockSkew as Config.ClockSkew disables the tolerance for wallet clocks.
const NoClockSkew time.Duration = -1

// SignatureVerifier verifies signatures of one CAIP-2 namespace. A returned error means
// the check could not be completed.
type SignatureVerifier interface {
	Namespace() string
	Verify(ctx context.Context, message *Message, signature string) (bool, error)
	NormaliseAddress(address string) string
}

// NonceStore issues single-use nonces. Consume must be atomic: of two parallel calls
// with one nonce, exactly one succeeds, and the other returns ErrInvalidNonce.
type NonceStore interface {
	Issue(ctx context.Context) (string, error)
	Exists(ctx context.Context, nonce string) (bool, error)
	Consume(ctx context.Context, nonce string) error
}

// Config configures a Verifier. Nonces and Verifiers are required, and an empty Domains
// rejects every message.
type Config struct {
	// Domains lists allowed domains exactly as wallets send them, with a port if any.
	Domains []string
	// ClockSkew tolerates wallet clocks running ahead. Zero means five minutes.
	ClockSkew time.Duration
	Nonces    NonceStore
	// Verifiers lists the enabled namespaces, such as evm.New(nil) and solana.New().
	Verifiers []SignatureVerifier
	// Now replaces time.Now.
	Now func() time.Time
}

// Session is the result of a successful verification.
type Session struct {
	Address   string
	Namespace string
	ChainID   string
	Domain    string
	IssuedAt  time.Time
	Message   *Message
}

// Verifier checks signed messages. It is safe for concurrent use.
type Verifier struct {
	domains   []string
	verifiers map[string]SignatureVerifier
	nonces    NonceStore
	clockSkew time.Duration
	now       func() time.Time
}

// New returns a Verifier for the given configuration.
func New(config Config) (*Verifier, error) {
	if config.Nonces == nil {
		return nil, errors.New("siwx: a nonce store is required")
	}
	if len(config.Verifiers) == 0 {
		return nil, errors.New("siwx: at least one verifier is required")
	}

	verifier := &Verifier{
		domains:   normaliseDomains(config.Domains),
		verifiers: map[string]SignatureVerifier{},
		nonces:    config.Nonces,
		clockSkew: resolveClockSkew(config.ClockSkew),
		now:       config.Now,
	}

	if verifier.now == nil {
		verifier.now = time.Now
	}

	for _, registered := range config.Verifiers {
		verifier.verifiers[registered.Namespace()] = registered
	}

	return verifier, nil
}

// Verify checks a signed message and consumes its nonce. Rejections wrap a *Error; an
// outage of the contract check also wraps its cause, and a store failure comes through as is.
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

	valid, err := chainVerifier.Verify(ctx, message, signature)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidSignature, err)
	}
	if !valid {
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

func resolveClockSkew(configured time.Duration) time.Duration {
	if configured == 0 {
		return defaultClockSkew
	}

	return max(configured, 0)
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
