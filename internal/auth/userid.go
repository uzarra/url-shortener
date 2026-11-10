package auth

import "uuid"

func NewUserID() string {
	return uuid.New().String()
}
