package siwx

type Error struct {
	Code string
}

func (failure *Error) Error() string {
	return failure.Code
}

var (
	ErrInvalidMessage       = &Error{Code: "siwx_invalid_message"}
	ErrInvalidDomain        = &Error{Code: "siwx_invalid_domain"}
	ErrInvalidNonce         = &Error{Code: "siwx_invalid_nonce"}
	ErrInvalidSignature     = &Error{Code: "siwx_invalid_signature"}
	ErrUnsupportedNamespace = &Error{Code: "siwx_unsupported_namespace"}
)
