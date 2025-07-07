package models

type Group struct {
	ID          string `json:"uuid"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

type GroupWithUsers struct {
	Group
	Users []User
}
