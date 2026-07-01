package errors

// ErrorResponse is the standard API error payload shared by IAM and gateway.
type ErrorResponse struct {
	Status    int    `json:"status"`
	Code      string `json:"code"`
	Message   string `json:"message"`
	Detail    any    `json:"detail,omitempty"`
	RequestID string `json:"request_id,omitempty"`
}

func NewErrorResponse(code, message string) *ErrorResponse {
	return &ErrorResponse{
		Code:    code,
		Message: message,
	}
}

func (e *ErrorResponse) WithStatus(status int) *ErrorResponse {
	e.Status = status
	return e
}

func (e *ErrorResponse) WithDetail(detail any) *ErrorResponse {
	e.Detail = detail
	return e
}

func (e *ErrorResponse) WithRequestID(requestID string) *ErrorResponse {
	e.RequestID = requestID
	return e
}

func (e *ErrorResponse) Error() string {
	return e.Message
}
