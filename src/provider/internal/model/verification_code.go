package model

type VerificationCode struct {
	ID        string `json:"id"`
	Phone     string `json:"phone"`
	Code      string `json:"code"`
	ExpiresAt string `json:"expires_at"`
	Used      bool   `json:"used"`
	CreatedAt string `json:"created_at"`
}
