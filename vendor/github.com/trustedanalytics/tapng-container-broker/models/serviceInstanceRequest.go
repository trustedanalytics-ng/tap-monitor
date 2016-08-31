package models

type CreateInstanceRequest struct {
	Image string `json:"image"`
}

type ScaleInstanceRequest struct {
	Replicas int `json:"replicas"`
}
