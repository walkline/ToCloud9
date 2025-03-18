package session

type UserFriendlyError struct {
	UserError string
	RealError error
}

func (e *UserFriendlyError) Error() string {
	return e.RealError.Error()
}

func NewMailServiceUnavailableErr(err error) error {
	return &UserFriendlyError{
		UserError: "Mailing service unavailable. Try again later.",
		RealError: err,
	}
}

func NewGroupServiceUnavailableErr(err error) error {
	return &UserFriendlyError{
		UserError: "Group service unavailable. Try again later.",
		RealError: err,
	}
}
