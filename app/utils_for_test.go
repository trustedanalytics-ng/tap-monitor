/**
 * Copyright (c) 2017 Intel Corporation
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

	"github.com/golang/mock/gomock"

	catalogModels "github.com/trustedanalytics/tap-catalog/models"
	"github.com/trustedanalytics/tap-monitor/mocks"
)

const (
	applicationID = "appID"
	offeringID    = "test-offering-id"
	planId        = "test-plan-id"
	planName      = "test-plan-name"
	templateID    = "test-template-id"
)

var (
	imageID             = catalogModels.GenerateImageId(applicationID)
	imageWithOfferingID = catalogModels.ConstructImageIdForUserOffering(offeringID)
)

type Mocker struct {
	kubernetesAPI *mocks.MockKubernetesApi
	catalogAPI    *mocks.MockTapCatalogApi
	templateAPI   *mocks.MockTemplateRepository
}

func prepareMockerAndMockedConnectionConfig(t *testing.T) (Mocker, *gomock.Controller) {
	mockCtrl := gomock.NewController(t)

	mocker := Mocker{
		kubernetesAPI: mocks.NewMockKubernetesApi(mockCtrl),
		catalogAPI:    mocks.NewMockTapCatalogApi(mockCtrl),
		templateAPI:   mocks.NewMockTemplateRepository(mockCtrl),
	}

	config = getMockedConnectionConfig(mocker)
	return mocker, mockCtrl
}

func getMockedConnectionConfig(mocker Mocker) *ConnectionConfig {
	return &ConnectionConfig{
		KubernetesApi:         mocker.kubernetesAPI,
		CatalogApi:            mocker.catalogAPI,
		TemplateRepositoryApi: mocker.templateAPI,
	}
}

func getSampleService() catalogModels.Service {
	servicePlan := catalogModels.ServicePlan{
		Id:   planId,
		Name: planName,
	}
	service := catalogModels.Service{
		Id:         offeringID,
		TemplateId: templateID,
		Plans:      []catalogModels.ServicePlan{servicePlan},
	}
	return service
}

func getSampleApplicationAndImage() (catalogModels.Application, catalogModels.Image) {
	image := catalogModels.Image{
		Id: imageID,
	}
	app := catalogModels.Application{
		Id:          applicationID,
		ImageId:     image.Id,
		TemplateId:  templateID,
		Replication: 1,
	}

	return app, image
}
