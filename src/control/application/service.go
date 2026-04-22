package application

import "github.com/google/uuid"

type (
	ApplicationId string
)

func NewApplicationId() ApplicationId {
	return ApplicationId(uuid.NewString())
}

// String returns the string representation of ApplicationId.
func (s ApplicationId) String() string {
	return string(s)
}

