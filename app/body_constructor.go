package app

import (
	"encoding/json"

	catalogModels "github.com/trustedanalytics/tap-catalog/models"
	containerBrokerModels "github.com/trustedanalytics/tap-container-broker/models"
	imageFactoryModels "github.com/trustedanalytics/tap-image-factory/models"
)

func getCreateInstanceRequest(instance catalogModels.Instance) containerBrokerModels.CreateInstanceRequest {
	return containerBrokerModels.CreateInstanceRequest{
		InstanceId: instance.Id,
		TemplateId: catalogModels.GetValueFromMetadata(instance.Metadata, catalogModels.BROKER_TEMPLATE_ID),
		Image:      catalogModels.GetValueFromMetadata(instance.Metadata, catalogModels.APPLICATION_IMAGE_ADDRESS),
	}
}

func getDeleteInstanceRequest(instance catalogModels.Instance) containerBrokerModels.DeleteRequest {
	return containerBrokerModels.DeleteRequest{
		Id: instance.Id,
	}
}

func getBuildImagePostRequest(image catalogModels.Image) imageFactoryModels.BuildImagePostRequest {
	return imageFactoryModels.BuildImagePostRequest{
		ImageId: image.Id,
	}
}

func prepareCreateInstanceRequest(instance catalogModels.Instance) ([]byte, error) {
	request := getCreateInstanceRequest(instance)
	return json.Marshal(request)
}

func prepareDeleteInstanceRequest(instance catalogModels.Instance) ([]byte, error) {
	request := getDeleteInstanceRequest(instance)
	return json.Marshal(request)
}

func prepareBuildImageRequest(image catalogModels.Image) ([]byte, error) {
	request := getBuildImagePostRequest(image)
	return json.Marshal(request)
}
