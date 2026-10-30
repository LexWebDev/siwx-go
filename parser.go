package siwx

import (
	"regexp"
	"slices"
	"strings"
	"time"
)

const (
	recapSentence  = "I further authorize the stated URI to perform the following actions on my behalf:"
	recapPrefix    = "urn:recap:"
	resourcesLabel = "Resources:"
	fieldSeparator = ": "
	trimmedRunes   = " \t\n\r\x00\x0b"
)

var (
	headerPattern = regexp.MustCompile(`^(?:([a-zA-Z][a-zA-Z0-9+.\-]*)://)?(\S+) wants you to sign in with your (.+) account:$`)
	caip2Pattern  = regexp.MustCompile(`^[-a-z0-9]{3,8}:[-_a-zA-Z0-9]{1,32}$`)
	digitsPattern = regexp.MustCompile(`^[0-9]+$`)
	noncePattern  = regexp.MustCompile(`^[a-zA-Z0-9]{8,}$`)
	rfc3339Format = regexp.MustCompile(`(?i)^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(\.\d+)?(Z|[+-]\d{2}:\d{2})$`)

	fieldKeys = []string{
		"URI", "Version", "Chain ID", "Nonce",
		"Issued At", "Expiration Time", "Not Before", "Request ID",
	}
)

// Parse reads an EIP-4361 message for any network. It rejects what the grammar forbids
// but leaves the address format to the namespace verifier.
func Parse(raw string) (*Message, error) {
	normalised := strings.Trim(strings.ReplaceAll(raw, "\r\n", "\n"), trimmedRunes)
	if strings.Contains(normalised, "\r") {
		return nil, ErrInvalidMessage
	}

	lines := strings.Split(normalised, "\n")

	header := headerPattern.FindStringSubmatch(lines[0])
	if header == nil || len(lines) < 2 {
		return nil, ErrInvalidMessage
	}

	address := strings.Trim(lines[1], trimmedRunes)
	if address == "" {
		return nil, ErrInvalidMessage
	}

	signedStatement, body := extractStatement(lines[2:])

	fields, resources, err := extractBody(body)
	if err != nil {
		return nil, err
	}

	message := &Message{
		Raw:             normalised,
		Scheme:          header[1],
		Domain:          header[2],
		Network:         header[3],
		Address:         address,
		Statement:       stripRecapSentence(signedStatement, resources),
		SignedStatement: signedStatement,
		RequestID:       fields["Request ID"],
		Resources:       resources,
	}

	if err := applyRequiredFields(message, fields); err != nil {
		return nil, err
	}

	if err := applyOptionalTimes(message, fields); err != nil {
		return nil, err
	}

	return message, nil
}

func extractStatement(lines []string) (string, []string) {
	for len(lines) > 0 && lines[0] == "" {
		lines = lines[1:]
	}

	if len(lines) == 0 {
		return "", lines
	}

	for _, key := range fieldKeys {
		if strings.HasPrefix(lines[0], key+fieldSeparator) {
			return "", lines
		}
	}

	return lines[0], lines[1:]
}

func extractBody(lines []string) (map[string]string, []string, error) {
	fields := map[string]string{}

	for index, line := range lines {
		if line == "" {
			continue
		}

		if line == resourcesLabel {
			resources, err := extractResources(lines[index+1:])

			return fields, resources, err
		}

		key, value, separated := strings.Cut(line, fieldSeparator)
		_, duplicate := fields[key]

		if !separated || duplicate || !slices.Contains(fieldKeys, key) {
			return nil, nil, ErrInvalidMessage
		}

		fields[key] = value
	}

	return fields, nil, nil
}

func extractResources(lines []string) ([]string, error) {
	resources := make([]string, 0, len(lines))

	for _, line := range lines {
		resource, listed := strings.CutPrefix(line, "- ")
		if !listed {
			return nil, ErrInvalidMessage
		}

		resources = append(resources, resource)
	}

	return resources, nil
}

func stripRecapSentence(statement string, resources []string) string {
	hasRecap := slices.ContainsFunc(resources, func(resource string) bool {
		return strings.HasPrefix(resource, recapPrefix)
	})
	if !hasRecap {
		return statement
	}

	base, _, found := strings.Cut(statement, recapSentence)
	if !found {
		return statement
	}

	return strings.TrimRight(base, trimmedRunes)
}

func applyRequiredFields(message *Message, fields map[string]string) error {
	for _, key := range []string{"URI", "Version", "Chain ID", "Nonce", "Issued At"} {
		if fields[key] == "" {
			return ErrInvalidMessage
		}
	}

	namespace, chainID, err := resolveChain(fields["Chain ID"])
	if err != nil {
		return err
	}

	if !noncePattern.MatchString(fields["Nonce"]) {
		return ErrInvalidMessage
	}

	issuedAt, err := parseTime(fields["Issued At"])
	if err != nil {
		return err
	}

	message.URI = fields["URI"]
	message.Version = fields["Version"]
	message.Namespace = namespace
	message.ChainID = chainID
	message.Nonce = fields["Nonce"]
	message.IssuedAt = issuedAt

	return nil
}

func applyOptionalTimes(message *Message, fields map[string]string) error {
	targets := map[string]**time.Time{
		"Expiration Time": &message.ExpirationTime,
		"Not Before":      &message.NotBefore,
	}

	for key, target := range targets {
		value, present := fields[key]
		if !present {
			continue
		}

		parsed, err := parseTime(value)
		if err != nil {
			return err
		}

		*target = &parsed
	}

	return nil
}

func resolveChain(value string) (string, string, error) {
	if strings.Contains(value, ":") {
		if !caip2Pattern.MatchString(value) {
			return "", "", ErrInvalidMessage
		}

		namespace, _, _ := strings.Cut(value, ":")

		return namespace, value, nil
	}

	if !digitsPattern.MatchString(value) {
		return "", "", ErrInvalidMessage
	}

	return "eip155", "eip155:" + value, nil
}

func parseTime(value string) (time.Time, error) {
	if !rfc3339Format.MatchString(value) {
		return time.Time{}, ErrInvalidMessage
	}

	parsed, err := time.Parse(time.RFC3339Nano, strings.ToUpper(value))
	if err != nil {
		return time.Time{}, ErrInvalidMessage
	}

	return parsed, nil
}
