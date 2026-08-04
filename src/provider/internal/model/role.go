package model

type Role struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Permissions []string `json:"permissions" gorm:"serializer:json"` // gorm: 序列化为 JSON（pg: jsonb / sqlite: text）
}
