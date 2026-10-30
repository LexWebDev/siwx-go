package siwx

// Error is a verification failure identified by a machine-readable code.
type Error struct {
	Code string
}

// Error returns the code.
func (failure *Error) Error() string {
	return failure.Code
}

// Failures returned by Parse and Verifier.Verify. Match them with errors.Is.
var (
	ErrInvalidMessage       = &Error{Code: "siwx_invalid_message"}
	ErrInvalidDomain        = &Error{Code: "siwx_invalid_domain"}
	ErrInvalidNonce         = &Error{Code: "siwx_invalid_nonce"}
	ErrInvalidSignature     = &Error{Code: "siwx_invalid_signature"}
	ErrUnsupportedNamespace = &Error{Code: "siwx_unsupported_namespace"}
)
