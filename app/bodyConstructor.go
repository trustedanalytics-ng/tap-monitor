package app

import (
	"encoding/json"

	containerBrokerModels "github.com/trustedanalytics/tap-container-broker/models"
	catalogModels "github.com/trustedanalytics/tap-catalog/models"
)



func createServiceInstanceBody(instance catalogModels.Instance) ([]byte, error) {

	body := containerBrokerModels.ServiceInstanceBodyQueue{
		InstanceId: instance.Id,
	}
	body.Image = ""

	byted, err := json.Marshal(body)
	if err != nil {
		return []byte{}, err
	}

	return byted, nil
}
