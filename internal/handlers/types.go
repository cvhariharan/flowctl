package handlers

import "github.com/cvhariharan/autopilot/internal/core/models"

type AuthReq struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type ApprovalRequestResp struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	RequestedBy string `json:"requested_by"`
}

type ApprovalActionResp struct {
	ID      string `json:"id"`
	Status  string `json:"status"`
	Message string `json:"messages"`
}

type User struct {
	ID        string `json:"id"`
	Username  string `json:"username"`
	Name      string `json:"name"`
	LoginType string `json:"login_type"`
	Role      string `json:"role"`
}

type UserWithGroups struct {
	User
	Groups []Group `json:"groups"`
}

type Group struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Users       []User `json:"users"`
}

type GroupWithUsers struct {
	Group
	Users []User `json:"users"`
}

func coreUsertoUser(u models.User) User {
	return User{
		ID:        u.ID,
		Name:      u.Name,
		Username:  u.Username,
		LoginType: string(u.LoginType),
		Role:      string(u.Role),
	}
}

func coreGroupArrayCast(gu []models.Group) []Group {
	g := make([]Group, 0)
	for _, v := range gu {
		g = append(g, coreGroupToGroup(v))
	}
	return g
}

func coreUserArrayCast(gu []models.User) []User {
	u := make([]User, 0)
	for _, v := range gu {
		u = append(u, coreUsertoUser(v))
	}
	return u
}

func coreGroupToGroup(gu models.Group) Group {
	return Group{
		ID:          gu.ID,
		Name:        gu.Name,
		Description: gu.Description,
	}
}

type FlowInputValidationError struct {
	FieldName  string `json:"field"`
	ErrMessage string `json:"error"`
}

type FlowLogResp struct {
	MType   string            `json:"message_type"`
	Value   string            `json:"value"`
	Results map[string]string `json:"results,omitempty"`
}

type PaginateRequest struct {
	Filter string `query:"filter"`
	Page  int `query:"page"`
	Count int `query:"count_per_page"`
}

type UsersPaginateResponse struct {
	Users      []UserWithGroups `json:"users"`
	PageCount  int64            `json:"page_count"`
	TotalCount int64            `json:"total_count"`
}

type GroupsPaginateResponse struct {
	Groups      []GroupWithUsers `json:"groups"`
	PageCount  int64            `json:"page_count"`
	TotalCount int64            `json:"total_count"`
}
