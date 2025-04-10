package errors

type ErrorCode string

type ServiceError struct {
	Code    ErrorCode
	Message string
	Err     error
}

func (se ServiceError) Error() string {
	return se.Message
}

func (se ServiceError) Unwrap() error {
	return se.Err
}
