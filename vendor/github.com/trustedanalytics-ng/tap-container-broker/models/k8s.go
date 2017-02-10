package models

import "k8s.io/kubernetes/pkg/apis/extensions"

type PrepareDeployment func() (*extensions.Deployment, error)
