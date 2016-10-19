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
	"os"
	"sync"
	"time"

	"github.com/streadway/amqp"

	catalogModels "github.com/trustedanalytics/tap-catalog/models"
	containerBrokerModels "github.com/trustedanalytics/tap-container-broker/models"
	"github.com/trustedanalytics/tap-go-common/logger"
	"github.com/trustedanalytics/tap-go-common/queue"
	"github.com/trustedanalytics/tap-go-common/util"
	imageFactoryModels "github.com/trustedanalytics/tap-image-factory/models"
)

var logger = logger_wrapper.InitLogger("app")
var docker_hub_address = os.Getenv("IMAGE_FACTORY_HUB_ADDRESS")

const checkInternalSeconds = 5
const imagesRepoUriPlaceHolder = "{{ repository_uri }}"

// todo this is temporary -> DPNG-10694
var createInstanceQueue = []string{}
var deleteInstanceQueue = []string{}
var pendingImageQueue = []string{}
var readyImageQueue = []string{}

var genericServiceTemplateID = os.Getenv("GENERIC_SERVICE_TEMPLATE_ID")

type QueueManager struct {
	*amqp.Channel
	*amqp.Connection
}

func StartMonitor(waitGroup *sync.WaitGroup) {
	waitGroup.Add(1)

	channel, conn := setupQueueConnection()
	queueManager := &QueueManager{channel, conn}
	defer queueManager.Connection.Close()
	defer queueManager.Channel.Close()

	monitoringLoop(queueManager)
	waitGroup.Done()
}

func monitoringLoop(queueManager *QueueManager) {
	for {
		select {
		case <-time.After(checkInternalSeconds * time.Second):
			if err := queueManager.CheckCatalogRequestedInstances(); err != nil {
				logger.Error("Proccessing Catalog instances error:", err)
			}
			if err := queueManager.CheckCatalogRequestedImages(); err != nil {
				logger.Error("Proccessing Catalog images error:", err)
			}
		case <-util.GetTerminationObserverChannel():
			logger.Info("Monitoring stopped")
			return
		}
	}
}

func setupQueueConnection() (*amqp.Channel, *amqp.Connection) {
	channel, conn := queue.GetConnectionChannel()
	queue.CreateExchangeWithQueueByRoutingKeys(channel, containerBrokerModels.CONTAINER_BROKER_QUEUE_NAME,
		[]string{containerBrokerModels.CONTAINER_BROKER_CREATE_ROUTING_KEY, containerBrokerModels.CONTAINER_BROKER_DELETE_ROUTING_KEY})
	queue.CreateExchangeWithQueueByRoutingKeys(channel, imageFactoryModels.IMAGE_FACTORY_QUEUE_NAME, []string{imageFactoryModels.IMAGE_FACTORY_IMAGE_ROUTING_KEY})
	return channel, conn
}

func convertDependenciesToBindings(dependencies []catalogModels.InstanceDependency) []catalogModels.InstanceBindings {
	result := make([]catalogModels.InstanceBindings, len(dependencies))
	for i, dependency := range dependencies {
		result[i].Id = dependency.Id
	}

	return result
}

func (q *QueueManager) CheckCatalogRequestedImages() error {
	images, _, err := config.CatalogApi.ListImages()
	if err != nil {
		logger.Error("Failed to ListImages:", err)
		return err
	}

	for _, image := range images {
		switch image.State {
		case catalogModels.ImageStatePending:
			if isInstanceIdInArray(pendingImageQueue, image.Id) {
				continue
			}
			q.sendToQueueImageFactoryBuild(image)
			pendingImageQueue = append(pendingImageQueue, image.Id)

		case catalogModels.ImageStateReady:
			if isInstanceIdInArray(readyImageQueue, image.Id) {
				continue
			}
			readyImageQueue = append(readyImageQueue, image.Id)

			if catalogModels.IsApplicationInstance(image.Id) {
				err := ExecuteFlowForUserDefinedApp(image)
				if err != nil {
					break
				}
			}

			if catalogModels.IsUserDefinedOffering(image.Id) {
				err = ExecuteFlowForUserDefinedOffering(image)
				if err != nil {
					break
				}
			}
		}
	}
	return nil
}

func ExecuteFlowForUserDefinedApp(image catalogModels.Image) error {
	applicationId := catalogModels.GetApplicationId(image.Id)
	instances, _, err := config.CatalogApi.ListApplicationInstances(applicationId)
	if err != nil {
		logger.Errorf("Failed to get instances of application with id %s and image id %s", applicationId, image.Id)
		return err
	}

	if len(instances) == 0 {
		application, _, err := config.CatalogApi.GetApplication(applicationId)
		if err != nil {
			logger.Error("Failed to call GetApplication for image: ", image.Id, err)
			return err
		}

		instance := catalogModels.Instance{
			Name:     application.Name,
			Type:     catalogModels.InstanceTypeApplication,
			ClassId:  application.Id,
			Bindings: convertDependenciesToBindings(application.InstanceDependencies),
			Metadata: []catalogModels.Metadata{
				{Id: catalogModels.APPLICATION_IMAGE_ADDRESS, Value: getImageAddress(image.Id)},
			},
		}

		if _, _, err = config.CatalogApi.AddApplicationInstance(application.Id, instance); err != nil {
			logger.Errorf("Failed to call AddApplicationInstance for image: %s - err: %v", image.Id, err)
			return err
		}
	}
	return nil
}

func ExecuteFlowForUserDefinedOffering(image catalogModels.Image) error {
	newTemplate, _, err := config.TemplateRepositoryApi.GetTemplate(genericServiceTemplateID)
	if err != nil {
		logger.Errorf("cannot fetch generic template with id %s from Template Repository", genericServiceTemplateID)
		return err
	}

	emptyTemplate := catalogModels.Template{}
	emptyTemplate.State = catalogModels.TemplateStateInProgress
	templateEntryFromCatalog, _, err := config.CatalogApi.AddTemplate(emptyTemplate)
	if err != nil {
		logger.Errorf("cannot create template entry to catalog - err: %v", err)
		return err
	}

	newImageForTemplate := imagesRepoUriPlaceHolder + "/" + image.Id
	newTemplate.Body.Deployments[0].Spec.Template.Spec.Containers[0].Image = newImageForTemplate
	newTemplate.Id = templateEntryFromCatalog.Id

	_, err = config.TemplateRepositoryApi.CreateTemplate(newTemplate)
	if err != nil {
		logger.Errorf("cannot create template with id %s in Template Repository", newTemplate.Id)
		return err
	}

	_, _, err = UpdateTemplate(newTemplate.Id, "State", catalogModels.TemplateStateInProgress, catalogModels.TemplateStateReady)
	if err != nil {
		logger.Errorf("cannot update state of template with id %s", newTemplate.Id)
		return err
	}

	offeringID := catalogModels.GetOfferingId(image.Id)

	offering, _, err := config.CatalogApi.GetService(offeringID)
	if err != nil {
		logger.Errorf("cannot fetch service with id %s from Template Repository", offering.Id)
		return err
	}

	offering, _, err = UpdateOffering(offeringID, "TemplateId", offering.TemplateId, newTemplate.Id)
	if err != nil {
		logger.Errorf("cannot update service with id %s with new templateId", offering.Id)
		return err
	}

	offering, _, err = UpdateOffering(offeringID, "State", catalogModels.ServiceStateDeploying, catalogModels.ServiceStateReady)
	if err != nil {
		logger.Errorf("cannot update state of service with id %s", offering.Id)
		return err
	}
	return nil
}

func UpdateTemplate(serviceId string, keyNameToUpdate string, oldVal interface{}, newVal interface{}) (catalogModels.Template, int, error) {
	marshaledOldStateValue, err := json.Marshal(oldVal)
	if err != nil {
		logger.Errorf("cannot marshal value %s", oldVal)
		return catalogModels.Template{}, http.StatusBadRequest, err
	}

	marshaledNewStateValue, err := json.Marshal(newVal)
	if err != nil {
		logger.Errorf("cannot marshal value %s", oldVal)
		return catalogModels.Template{}, http.StatusBadRequest, err
	}

	patches := []catalogModels.Patch{{catalogModels.OperationUpdate, keyNameToUpdate, marshaledNewStateValue, marshaledOldStateValue}}
	return config.CatalogApi.UpdateTemplate(serviceId, patches)
}

func UpdateOffering(serviceId string, keyNameToUpdate string, oldVal interface{}, newVal interface{}) (catalogModels.Service, int, error) {
	marshaledOldStateValue, err := json.Marshal(oldVal)
	if err != nil {
		logger.Errorf("cannot marshal value %s", oldVal)
		return catalogModels.Service{}, http.StatusBadRequest, err
	}

	marshaledNewStateValue, err := json.Marshal(newVal)
	if err != nil {
		logger.Errorf("cannot marshal value %s", oldVal)
		return catalogModels.Service{}, http.StatusBadRequest, err
	}

	patches := []catalogModels.Patch{{catalogModels.OperationUpdate, keyNameToUpdate, marshaledNewStateValue, marshaledOldStateValue}}
	return config.CatalogApi.UpdateService(serviceId, patches)
}

func getImageAddress(id string) string {
	return docker_hub_address + "/" + id
}

func (q *QueueManager) CheckCatalogRequestedInstances() error {
	instances, _, err := config.CatalogApi.ListInstances()
	if err != nil {
		return err
	}

	for _, instance := range instances {
		if instance.State == catalogModels.InstanceStateRequested {
			if isInstanceIdInArray(createInstanceQueue, instance.Id) {
				continue
			}
			q.sendToQueueBrokerCreate(instance)
			createInstanceQueue = append(createInstanceQueue, instance.Id)
		} else if instance.State == catalogModels.InstanceStateDestroyReq {
			if isInstanceIdInArray(deleteInstanceQueue, instance.Id) {
				continue
			}
			q.sendToQueueBrokerDelete(instance)
			deleteInstanceQueue = append(deleteInstanceQueue, instance.Id)
		}
	}
	return nil
}

func (q *QueueManager) sendToQueueBrokerCreate(instance catalogModels.Instance) {
	queueMessage, err := prepareCreateInstanceRequest(instance)
	q.sendMessageOnQueue(queueMessage, err,
		containerBrokerModels.CONTAINER_BROKER_QUEUE_NAME,
		containerBrokerModels.CONTAINER_BROKER_CREATE_ROUTING_KEY,
		"Failed to prepare CreateInstanceRequest for instance: ", instance.Id, err)
}

func (q *QueueManager) sendToQueueBrokerDelete(instance catalogModels.Instance) {
	queueMessage, err := prepareDeleteInstanceRequest(instance)
	q.sendMessageOnQueue(queueMessage, err,
		containerBrokerModels.CONTAINER_BROKER_QUEUE_NAME,
		containerBrokerModels.CONTAINER_BROKER_DELETE_ROUTING_KEY,
		"Failed to prepare DeleteRequest for instance: ", instance.Id, err)
}

func (q *QueueManager) sendToQueueImageFactoryBuild(image catalogModels.Image) {
	queueMessage, err := prepareBuildImageRequest(image)
	q.sendMessageOnQueue(queueMessage, err,
		imageFactoryModels.IMAGE_FACTORY_QUEUE_NAME,
		imageFactoryModels.IMAGE_FACTORY_IMAGE_ROUTING_KEY,
		"Failed to prepare BuildImagePostRequest for image: ", image.Id, err)
}

func (q *QueueManager) sendMessageOnQueue(queueMessage []byte, err error, queueName, routingKey string, errorMsg ...interface{}) {
	if err != nil {
		logger.Error(errorMsg...)
		// TODO question about ImageFactory break statement
		return
	}
	q.sendMessageAndReconnectIfError(queueMessage, queueName, routingKey)
}

func (q *QueueManager) sendMessageAndReconnectIfError(message []byte, queueName, routingKey string) {
	if err := queue.SendMessageToQueue(q.Channel, message, queueName, routingKey); err != nil {
		logger.Errorf("Can't send message on queue - err: %v! Trying to reconnect...", err)
		channel, conn := setupQueueConnection()
		q.Channel = channel
		q.Connection = conn

		if err := queue.SendMessageToQueue(q.Channel, message, queueName, routingKey); err != nil {
			logger.Fatalf("Can't send message on queue! err: %v", err)
		}
	}
}

// this method supposed to call directly to ETCD (using long pool) for check if specific instance status will change in e.g. next 10 minutes
// if this happen then we can stop observing isntnace in othercase we should try X more times
func isInstanceIdInArray(instanceIds []string, wantedInstanceId string) bool {
	for _, instanceId := range instanceIds {
		if instanceId == wantedInstanceId {
			return true
		}
	}
	return false
}
