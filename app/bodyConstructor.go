package app

import (
	"encoding/json"

	catalogModels "github.com/trustedanalytics/tap-catalog/models"
	containerBrokerModels "github.com/trustedanalytics/tap-container-broker/models"
	imageFactoryModels "github.com/trustedanalytics/tap-image-factory/models"
)

func prepareCreateInstanceRequest(instance catalogModels.Instance) ([]byte, error) {
	body := containerBrokerModels.CreateInstanceRequest{
		InstanceId: instance.Id,
		TemplateId: catalogModels.GetValueFromMetadata(instance.Metadata, catalogModels.BROKER_TEMPLATE_ID),
		Image:      catalogModels.GetValueFromMetadata(instance.Metadata, catalogModels.APPLICATION_IMAGE_ADDRESS),
	}

	byted, err := json.Marshal(body)
	if err != nil {
		return []byte{}, err
	}

	return byted, nil
}

func prepareDeleteInstanceRequest(instance catalogModels.Instance) ([]byte, error) {
	body := containerBrokerModels.DeleteRequest{
		Id: instance.Id,
	}

	byted, err := json.Marshal(body)
	if err != nil {
		return []byte{}, err
	}

	return byted, nil
}

func prepareBuildImageRequest(image catalogModels.Image) ([]byte, error) {
	body := imageFactoryModels.BuildImagePostRequest{
		ImageId: image.Id,
	}

	byted, err := json.Marshal(body)
	if err != nil {
		return []byte{}, err
	}

	return byted, nil
}
