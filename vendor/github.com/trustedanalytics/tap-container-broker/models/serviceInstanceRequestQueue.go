package models

type ServiceInstanceBodyQueue struct {
	CreateInstanceRequest
	InstanceId	string `json:"instanceId"`
}
