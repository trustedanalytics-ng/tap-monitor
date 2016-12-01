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
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	catalogModels "github.com/trustedanalytics/tap-catalog/models"
	containerBrokerModels "github.com/trustedanalytics/tap-container-broker/models"
	imageFactoryModels "github.com/trustedanalytics/tap-image-factory/models"
)

const testInstanceID = "123"
const testBrokerTemplateID = "789"
const testApplicationImageAddress = "987"

func getTestInstance() catalogModels.Instance {
	return catalogModels.Instance{
		Id:       testInstanceID,
		Name:     "sample_instance",
		Type:     catalogModels.InstanceTypeApplication,
		ClassId:  "456",
		Bindings: []catalogModels.InstanceBindings{},
		Metadata: []catalogModels.Metadata{
			{Id: catalogModels.BROKER_TEMPLATE_ID, Value: testBrokerTemplateID},
			{Id: catalogModels.APPLICATION_IMAGE_ADDRESS, Value: testApplicationImageAddress},
		},
		State:      catalogModels.InstanceStateRequested,
		AuditTrail: catalogModels.AuditTrail{},
	}
}

const testImageID = "654"

func getTestImage() catalogModels.Image {
	return catalogModels.Image{
		Id:         testImageID,
		Type:       catalogModels.ImageTypeJava,
		State:      catalogModels.ImageStateBuilding,
		AuditTrail: catalogModels.AuditTrail{},
	}
}

func TestGetCreateInstanceRequest(t *testing.T) {
	Convey("getCreateInstanceRequest should return proper response", t, func() {
		testInstance := getTestInstance()
		properResponse := containerBrokerModels.CreateInstanceRequest{
			InstanceId: testInstanceID,
			TemplateId: testBrokerTemplateID,
			Image:      testApplicationImageAddress,
		}

		actualResponse := getCreateInstanceRequest(testInstance)

		So(actualResponse, ShouldResemble, properResponse)
	})
}

func TestGetDeleteInstanceRequest(t *testing.T) {
	Convey("getDeleteInstanceRequest should return proper response", t, func() {
		properResponse := containerBrokerModels.DeleteRequest{
			Id: testInstanceID,
		}

		actualResponse := getDeleteInstanceRequest(testInstanceID)

		So(actualResponse, ShouldResemble, properResponse)
	})
}

func TestGetBuildImagePostRequest(t *testing.T) {
	Convey("getBuildImagePostRequest should return proper response", t, func() {
		properResponse := imageFactoryModels.BuildImagePostRequest{
			ImageId: testImageID,
		}

		actualResponse := getBuildImagePostRequest(testImageID)

		So(actualResponse, ShouldResemble, properResponse)
	})
}
