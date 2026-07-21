package errors

const (
	CodeInvalidRequest        = "INVALID_REQUEST"
	CodeUnauthorized          = "UNAUTHORIZED"
	CodeForbidden             = "FORBIDDEN"
	CodeNotFound              = "NOT_FOUND"
	CodeRouteNotFound         = "ROUTE_NOT_FOUND"
	CodeMethodNotAllowed      = "METHOD_NOT_ALLOWED"
	CodeConflict              = "CONFLICT"
	CodeInternalError         = "INTERNAL_ERROR"
	CodeServiceUnavailable    = "SERVICE_UNAVAILABLE"
	CodeURITooLong            = "URI_TOO_LONG"
	CodeBodyTooLarge          = "BODY_TOO_LARGE"
	CodeInvalidEmail          = "INVALID_EMAIL"
	CodeInvalidUsername       = "INVALID_USERNAME"
	CodeInvalidPassword       = "INVALID_PASSWORD"
	CodeUserExists            = "USER_EXISTS"
	CodeUserNotFound          = "USER_NOT_FOUND"
	CodeUserDisabled          = "USER_DISABLED"
	CodeAuthFailed            = "AUTH_FAILED"
	CodeTokenExpired          = "TOKEN_EXPIRED"
	CodeTokenInvalid          = "TOKEN_INVALID"
	CodeRateLimitExceeded     = "RATE_LIMIT_EXCEEDED"
	CodeMissingAuthHeader     = "MISSING_AUTH_HEADER"
	CodeInvalidAuthHeader     = "INVALID_AUTH_HEADER"
	CodeEmptyToken            = "EMPTY_TOKEN"
	CodeInvalidTokenType      = "INVALID_TOKEN_TYPE"
	CodeMissingChallenge      = "MISSING_CHALLENGE"
	CodeInvalidChallenge      = "INVALID_CHALLENGE"
	CodeMissingUserID         = "MISSING_USER_ID"
	CodeSamePassword          = "SAME_PASSWORD"
	CodeWebAuthnNoCredentials = "WEBAUTHN_NO_CREDENTIALS"
	CodeInvalidQRCode         = "INVALID_QR_CODE"
	CodeMissingQRCode         = "MISSING_QR_CODE"
	CodeQRCodeConflict        = "QRCODE_CONFLICT"
)

var (
	ErrInvalidRequest = &ErrorResponse{
		Code:    CodeInvalidRequest,
		Message: "invalid request body",
	}

	ErrUnauthorized = &ErrorResponse{
		Code:    CodeUnauthorized,
		Message: "unauthorized",
	}

	ErrForbidden = &ErrorResponse{
		Code:    CodeForbidden,
		Message: "permission denied",
	}

	ErrNotFound = &ErrorResponse{
		Code:    CodeNotFound,
		Message: "resource not found",
	}

	ErrInternalError = &ErrorResponse{
		Code:    CodeInternalError,
		Message: "internal server error",
	}

	// ErrServiceUnavailable indicates the server cannot fulfil the request
	// because an internal dependency (typically the role database) is
	// temporarily unavailable. Distinct from ErrForbidden: a 503 means
	// "I cannot verify your permissions", NOT "you don't have permission".
	// Clients are expected to retry rather than treat it as account suspension.
	ErrServiceUnavailable = &ErrorResponse{
		Code:    CodeServiceUnavailable,
		Message: "service temporarily unavailable",
	}
)

func InvalidEmail() *ErrorResponse {
	return &ErrorResponse{
		Code:    CodeInvalidEmail,
		Message: "invalid email format",
	}
}

func InvalidUsername() *ErrorResponse {
	return &ErrorResponse{
		Code:    CodeInvalidUsername,
		Message: "invalid username format",
	}
}

func InvalidPassword() *ErrorResponse {
	return &ErrorResponse{
		Code:    CodeInvalidPassword,
		Message: "password must be a valid SHA3-256 hash",
	}
}

func UserExists() *ErrorResponse {
	return &ErrorResponse{
		Code:    CodeUserExists,
		Message: "email or username already registered",
	}
}

func UserNotFound() *ErrorResponse {
	return &ErrorResponse{
		Code:    CodeUserNotFound,
		Message: "user not found",
	}
}

func UserDisabled() *ErrorResponse {
	return &ErrorResponse{
		Code:    CodeUserDisabled,
		Message: "user is disabled",
	}
}

func AuthFailed() *ErrorResponse {
	return &ErrorResponse{
		Code:    CodeAuthFailed,
		Message: "invalid credentials",
	}
}

func TokenExpired() *ErrorResponse {
	return &ErrorResponse{
		Code:    CodeTokenExpired,
		Message: "token expired",
	}
}

func TokenInvalid() *ErrorResponse {
	return &ErrorResponse{
		Code:    CodeTokenInvalid,
		Message: "invalid token",
	}
}

func RateLimitExceeded() *ErrorResponse {
	return &ErrorResponse{
		Code:    CodeRateLimitExceeded,
		Message: "rate limit exceeded",
	}
}

func MissingAuthHeader() *ErrorResponse {
	return &ErrorResponse{
		Code:    CodeMissingAuthHeader,
		Message: "missing authorization header",
	}
}

func InvalidAuthHeader() *ErrorResponse {
	return &ErrorResponse{
		Code:    CodeInvalidAuthHeader,
		Message: "invalid authorization header format",
	}
}

func EmptyToken() *ErrorResponse {
	return &ErrorResponse{
		Code:    CodeEmptyToken,
		Message: "empty bearer token",
	}
}

func InvalidTokenType() *ErrorResponse {
	return &ErrorResponse{
		Code:    CodeInvalidTokenType,
		Message: "invalid token type",
	}
}

func MissingChallenge(challengeType string) *ErrorResponse {
	return &ErrorResponse{
		Code:    CodeMissingChallenge,
		Message: "missing " + challengeType + "_challenge",
	}
}

func InvalidChallenge(challengeType string) *ErrorResponse {
	return &ErrorResponse{
		Code:    CodeInvalidChallenge,
		Message: "invalid " + challengeType + "_challenge",
	}
}

func MissingUserID() *ErrorResponse {
	return &ErrorResponse{
		Code:    CodeMissingUserID,
		Message: "missing user id",
	}
}

func WebAuthnNoCredentials() *ErrorResponse {
	return &ErrorResponse{
		Code:    CodeWebAuthnNoCredentials,
		Message: "no webauthn credentials registered for this user",
	}
}

func MethodNotAllowed() *ErrorResponse {
	return &ErrorResponse{
		Code:    CodeMethodNotAllowed,
		Message: "method not allowed",
	}
}

func RouteNotFound() *ErrorResponse {
	return &ErrorResponse{
		Code:    CodeRouteNotFound,
		Message: "route not found",
	}
}

func InvalidQRCode() *ErrorResponse {
	return &ErrorResponse{
		Code:    CodeInvalidQRCode,
		Message: "invalid or expired qr code",
	}
}

func MissingQRCode() *ErrorResponse {
	return &ErrorResponse{
		Code:    CodeMissingQRCode,
		Message: "missing qr code parameter",
	}
}

func QRCodeConflict() *ErrorResponse {
	return &ErrorResponse{
		Code:    CodeQRCodeConflict,
		Message: "qr code state conflict",
	}
}

func URITooLong() *ErrorResponse {
	return &ErrorResponse{
		Code:    CodeURITooLong,
		Message: "request URI exceeds maximum length",
	}
}

func BodyTooLarge() *ErrorResponse {
	return &ErrorResponse{
		Code:    CodeBodyTooLarge,
		Message: "request body exceeds maximum size",
	}
}
