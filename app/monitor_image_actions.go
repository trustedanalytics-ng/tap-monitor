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
	"encoding/json"
	"net/http"
	"strings"

	"github.com/trustedanalytics/tap-catalog/builder"
	catalogModels "github.com/trustedanalytics/tap-catalog/models"
	templateRepositoryModels "github.com/trustedanalytics/tap-template-repository/model"
)

func convertDependenciesToBindings(dependencies []catalogModels.InstanceDependency) []catalogModels.InstanceBindings {
	result := make([]catalogModels.InstanceBindings, len(dependencies))
	for i, dependency := range dependencies {
		result[i].Id = dependency.Id
	}
	return result
}

func ExecuteFlowForUserDefinedApp(imageId string) error {
	applicationId := catalogModels.GetApplicationId(imageId)
	instances, _, err := config.CatalogApi.ListApplicationInstances(applicationId)
	if err != nil {
		logger.Errorf("Failed to get instances of application with id %s and image id %s", applicationId, imageId)
		return err
	}

	if len(instances) == 0 {
		application, _, err := config.CatalogApi.GetApplication(applicationId)
		if err != nil {
			logger.Error("Failed to call GetApplication for image: ", imageId, err)
			return err
		}

		instance := catalogModels.Instance{
			Name:     application.Name,
			Type:     catalogModels.InstanceTypeApplication,
			ClassId:  application.Id,
			Bindings: convertDependenciesToBindings(application.InstanceDependencies),
			Metadata: []catalogModels.Metadata{
				{Id: catalogModels.APPLICATION_IMAGE_ADDRESS, Value: getImageAddress(imageId)},
			},
			AuditTrail: catalogModels.AuditTrail{
				LastUpdateBy: application.AuditTrail.LastUpdateBy,
				CreatedBy:    application.AuditTrail.CreatedBy,
			},
		}
		instance.Metadata = append(instance.Metadata, application.Metadata...)

		if _, _, err = config.CatalogApi.AddApplicationInstance(application.Id, instance); err != nil {
			logger.Errorf("Failed to call AddApplicationInstance for image: %s - err: %v", imageId, err)
			return err
		}
	}
	return nil
}

func ExecuteFlowForUserDefinedOffering(imageId string) (err error) {
	logger.Infof("started offering creation from image with id %s", imageId)
	offeringID := catalogModels.GetOfferingId(imageId)
	defer func() {
		if err == nil {
			_, _, err = UpdateOffering(offeringID, "State", catalogModels.ServiceStateDeploying, catalogModels.ServiceStateReady)
			if err != nil {
				logger.Errorf("cannot update state of service with id %s from DEPLOYING to READY", offeringID)
			}
			logger.Infof("offering with id %s created successfully", offeringID)
		} else {
			logger.Errorf("error while creating offering with id %s from image with id %s", offeringID, imageId)
			_, _, err = UpdateOffering(offeringID, "State", catalogModels.ServiceStateDeploying, catalogModels.ServiceStateOffline)
			if err != nil {
				logger.Errorf("cannot update state of offering with id %s from DEPLOYING to OFFLINE", offeringID)
			}
		}
	}()

	newTemplate, _, err := config.TemplateRepositoryApi.GetRawTemplate(genericServiceTemplateID)
	if err != nil {
		logger.Errorf("cannot fetch generic template with id %s from Template Repository", genericServiceTemplateID)
		return err
	}

	image, _, err := config.CatalogApi.GetImage(imageId)
	if err != nil {
		logger.Error("GetImage error:", err)
		return err
	}

	emptyTemplate := catalogModels.Template{
		AuditTrail: catalogModels.AuditTrail{
			LastUpdateBy: image.AuditTrail.LastUpdateBy,
			CreatedBy:    image.AuditTrail.CreatedBy,
		},
	}
	emptyTemplate.State = catalogModels.TemplateStateInProgress
	templateEntryFromCatalog, _, err := config.CatalogApi.AddTemplate(emptyTemplate)
	if err != nil {
		logger.Errorf("cannot create template entry to catalog - err: %v", err)
		return err
	}

	newImageForTemplate := dockerHubAddress + "/" + image.Id
	templateID := templateEntryFromCatalog.Id

	newTemplate, err = adjustTemplateIdAndImage(templateID, newImageForTemplate, newTemplate)
	if err != nil {
		logger.Errorf("cannot adjust template id and image - template id: %s", templateID)
		return err
	}

	_, err = config.TemplateRepositoryApi.CreateTemplate(newTemplate)
	if err != nil {
		logger.Errorf("cannot create template with id %s in Template Repository", templateID)
		return err
	}

	_, _, err = UpdateTemplate(templateID, "State", catalogModels.TemplateStateInProgress, catalogModels.TemplateStateReady)
	if err != nil {
		logger.Errorf("cannot update state of template with id %s", templateID)
		return err
	}

	offering, _, err := config.CatalogApi.GetService(offeringID)
	if err != nil {
		logger.Errorf("cannot fetch service with id %s from catalog", offeringID)
		return err
	}

	offering, _, err = UpdateOffering(offeringID, "TemplateId", offering.TemplateId, templateID)
	if err != nil {
		logger.Errorf("cannot update service with id %s with new templateId equal to: %s", offeringID, templateID)
		return err
	}
	return
}

func adjustTemplateIdAndImage(templateID, image string, rawTemplate templateRepositoryModels.RawTemplate) (templateRepositoryModels.RawTemplate, error) {
	rawTemplate[templateRepositoryModels.RAW_TEMPLATE_ID_FIELD] = templateID

	rawTemplateBytes, err := json.Marshal(rawTemplate)
	if err != nil {
		return rawTemplate, err
	}

	adjustedRawTemplate := strings.Replace(string(rawTemplateBytes),
		templateRepositoryModels.GetPlaceholderWithDollarPrefix(templateRepositoryModels.PLACEHOLDER_IMAGE), image, -1)

	result := templateRepositoryModels.RawTemplate{}
	err = json.Unmarshal([]byte(adjustedRawTemplate), &result)
	return result, err
}

func UpdateTemplate(serviceId string, keyNameToUpdate string, oldVal interface{}, newVal interface{}) (catalogModels.Template, int, error) {
	patch, err := builder.MakePatchWithPreviousValue(keyNameToUpdate, oldVal, newVal, catalogModels.OperationUpdate)
	if err != nil {
		return catalogModels.Template{}, http.StatusBadRequest, err
	}

	return config.CatalogApi.UpdateTemplate(serviceId, []catalogModels.Patch{patch})
}

func UpdateOffering(serviceId string, keyNameToUpdate string, oldVal interface{}, newVal interface{}) (catalogModels.Service, int, error) {
	patch, err := builder.MakePatchWithPreviousValue(keyNameToUpdate, oldVal, newVal, catalogModels.OperationUpdate)
	if err != nil {
		return catalogModels.Service{}, http.StatusBadRequest, err
	}

	return config.CatalogApi.UpdateService(serviceId, []catalogModels.Patch{patch})
}

func getImageAddress(id string) string {
	return dockerHubAddress + "/" + id
}
