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
	"net/http"
	"testing"

	"github.com/golang/mock/gomock"
	. "github.com/smartystreets/goconvey/convey"

	"github.com/trustedanalytics-ng/tap-catalog/builder"
	catalogModels "github.com/trustedanalytics-ng/tap-catalog/models"
	templateModels "github.com/trustedanalytics-ng/tap-template-repository/model"
)

func TestExecuteFlowForUserDefinedApp(t *testing.T) {
	Convey("Test ExecuteFlowForUserDefinedApp", t, func() {
		mocker, mockCtrl := prepareMockerAndMockedConnectionConfig(t)

		application, image := getSampleApplicationAndImage()
		instance := createInstanceFromApplication(application, image.Id)

		Convey("Should create instance for application and return nil error response", func() {
			gomock.InOrder(
				mocker.catalogAPI.EXPECT().ListApplicationInstances(applicationID).Return([]catalogModels.Instance{}, http.StatusOK, nil),
				mocker.catalogAPI.EXPECT().GetApplication(applicationID).Return(application, http.StatusOK, nil),
				mocker.catalogAPI.EXPECT().AddApplicationInstance(applicationID, instance).Return(instance, http.StatusOK, nil),
			)

			err := ExecuteFlowForUserDefinedApp(imageID)
			So(err, ShouldBeNil)
		})

		Reset(func() {
			mockCtrl.Finish()
		})
	})
}

func TestExecuteFlowForUserDefinedOffering(t *testing.T) {
	Convey("Test ExecuteFlowForUserDefinedOffering", t, func() {
		mocker, mockCtrl := prepareMockerAndMockedConnectionConfig(t)

		service := getSampleService()

		_, image := getSampleApplicationAndImage()

		catalogTemplate := catalogModels.Template{Id: "catalogTemplateId"}

		patchOfferingTemplateId, _ := builder.MakePatchWithPreviousValue("TemplateId", catalogTemplate.Id, service.TemplateId, catalogModels.OperationUpdate)
		patchOfferingState, _ := builder.MakePatchWithPreviousValue("State", catalogModels.ServiceStateReady, catalogModels.ServiceStateDeploying, catalogModels.OperationUpdate)

		Convey("Should create instance for application and return nil error response", func() {
			gomock.InOrder(
				mocker.templateAPI.EXPECT().GetRawTemplate(genericServiceTemplateID).Return(templateModels.RawTemplate{}, http.StatusOK, nil),
				mocker.catalogAPI.EXPECT().GetImage(imageWithOfferingID).Return(image, http.StatusOK, nil),

				mocker.catalogAPI.EXPECT().AddTemplate(gomock.Any()).Return(catalogTemplate, http.StatusOK, nil),
				mocker.templateAPI.EXPECT().CreateTemplate(gomock.Any()).Return(http.StatusOK, nil),
				mocker.catalogAPI.EXPECT().UpdateTemplate(catalogTemplate.Id, gomock.Any()).Return(catalogTemplate, http.StatusOK, nil),

				mocker.catalogAPI.EXPECT().GetService(offeringID).Return(service, http.StatusOK, nil),
				mocker.catalogAPI.EXPECT().UpdateService(offeringID, []catalogModels.Patch{patchOfferingTemplateId}).Return(service, http.StatusOK, nil),
				mocker.catalogAPI.EXPECT().UpdateService(offeringID, []catalogModels.Patch{patchOfferingState}).Return(service, http.StatusOK, nil),
			)

			err := ExecuteFlowForUserDefinedOffering(imageWithOfferingID)
			So(err, ShouldBeNil)
		})

		Reset(func() {
			mockCtrl.Finish()
		})
	})
}
