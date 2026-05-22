package session

import "strings"

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
	userError := "Group service unavailable. Try again later."
	if isGroupPermissionError(err) {
		userError = "You are not the group leader or assistant."
	}

	return &UserFriendlyError{
		UserError: userError,
		RealError: err,
	}
}

func isGroupPermissionError(err error) bool {
	if err == nil {
		return false
	}

	return strings.Contains(strings.ToLower(err.Error()), "not enough permissions")
}

func isSilentGroupMutationError(err error) bool {
	if err == nil {
		return false
	}

	errMsg := strings.ToLower(err.Error())
	return strings.Contains(errMsg, "lfg group does not allow this operation") ||
		strings.Contains(errMsg, "invalid group operation")
}
