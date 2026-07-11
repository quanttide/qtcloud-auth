package model

type User struct {
	ID            string `json:"id"`
	Username      string `json:"username"`
	PasswordHash  string `json:"password_hash"`
	Phone         string `json:"phone"`
	PhoneVerified bool   `json:"phone_verified"`
	Nickname      string `json:"nickname"`
	Avatar        string `json:"avatar"`
	RoleID        string `json:"role_id"`
	CreatedAt     string `json:"created_at"`
}
