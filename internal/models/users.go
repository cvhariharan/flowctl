package models

import (
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

type UserLoginType string
type UserRoleType string

const (
	OIDCLoginType UserLoginType = "oidc"
	// Password based login
	StandardLoginType UserLoginType = "standard"

	AdminUserRole    UserRoleType = "admin"
	StandardUserRole UserRoleType = "user"
)

type UserInfo struct {
	ID       string   `json:"id"`
	Username string   `json:"email"`
	Name     string   `json:"name"`
	Groups   []string `json:"groups"`
}

type UserWithGroups struct {
	User
	Groups []Group
}

func (u UserWithGroups) GetUser() User {
	return u.User
}

func (u UserWithGroups) ToUserInfo() UserInfo {
	var groups []string
	for _, v := range u.Groups {
		groups = append(groups, v.ID)
	}

	return UserInfo{
		ID:       u.ID,
		Username: u.Username,
		Name:     u.Name,
		Groups:   groups,
	}
}

type User struct {
	ID        string        `json:"uuid"`
	Username  string        `json:"username"`
	Name      string        `json:"name"`
	LoginType UserLoginType `json:"login_type"`
	Role      UserRoleType  `json:"role"`
	password  string
}

// WithPassword sets the user password, the password should be hashed
func (u User) WithPassword(password string) User {
	u.password = password
	return u
}

func (u User) CheckPassword(password string) error {
	if err := bcrypt.CompareHashAndPassword([]byte(u.password), []byte(password)); err != nil {
		return fmt.Errorf("passwords don't match: %w", err)
	}

	return nil
}
