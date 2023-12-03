package chatlib

type Error string

func (e Error) Error() string {
	return string(e)
}

const (
	ErrInvalidConfig Error = "invalidConfig"
	ErrTimeout       Error = "timeout"
)
