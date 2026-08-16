package auth

import "fmt"

type Claims struct {
	Sub           string `json:"sub"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	Name          string `json:"name"`
	Picture       string `json:"picture"`
}

func (c Claims) Valid() error {
	if c.Sub == "" {
		return fmt.Errorf("Missing sub in claims")
	}
	return nil
}
