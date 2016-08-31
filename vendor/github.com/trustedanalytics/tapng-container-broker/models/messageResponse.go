package models

type DeployResponseStatus string

const (
	DeployResponseStatusSuccess              DeployResponseStatus = "success"
	DeployResponseStatusUnknown              DeployResponseStatus = "unknown"
	DeployResponseStatusInvalidConfiguration DeployResponseStatus = "invalid configuration"
	DeployResponseStatusInvalidState         DeployResponseStatus = "invalid state"
	DeployResponseStatusUnrecoverableError   DeployResponseStatus = "unrecoverable error"
)

type MessageResponse struct {
	Message DeployResponseStatus `json:"message"`
}
