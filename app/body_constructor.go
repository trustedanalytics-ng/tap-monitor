/**
 * Copyright (c) 2016 Intel Corporation
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *    http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package app

import (
	catalogModels "github.com/trustedanalytics-ng/tap-catalog/models"
	containerBrokerModels "github.com/trustedanalytics-ng/tap-container-broker/models"
	imageFactoryModels "github.com/trustedanalytics-ng/tap-image-factory/models"
)

func getCreateInstanceRequest(instance catalogModels.Instance) containerBrokerModels.CreateInstanceRequest {
	return containerBrokerModels.CreateInstanceRequest{
		InstanceId: instance.Id,
	}
}

func getDeleteInstanceRequest(instanceId string) containerBrokerModels.DeleteRequest {
	return containerBrokerModels.DeleteRequest{
		Id: instanceId,
	}
}

func getScaleInstanceRequest(instanceId string) containerBrokerModels.ScaleInstanceRequest {
	return containerBrokerModels.ScaleInstanceRequest{
		Id: instanceId,
	}
}

func getBuildImagePostRequest(imageId string) imageFactoryModels.BuildImagePostRequest {
	return imageFactoryModels.BuildImagePostRequest{
		ImageId: imageId,
	}
}
