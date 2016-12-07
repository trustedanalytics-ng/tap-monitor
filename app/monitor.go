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
	"strings"
	"sync"
	"time"

	"github.com/streadway/amqp"

	catalogModels "github.com/trustedanalytics/tap-catalog/models"
	containerBrokerModels "github.com/trustedanalytics/tap-container-broker/models"
	commonLogger "github.com/trustedanalytics/tap-go-common/logger"
	"github.com/trustedanalytics/tap-go-common/queue"
	"github.com/trustedanalytics/tap-go-common/util"
	imageFactoryModels "github.com/trustedanalytics/tap-image-factory/models"
	"github.com/trustedanalytics/tap-monitor/app/synchronization"
	templateRepositoryModels "github.com/trustedanalytics/tap-template-repository/model"
)

var logger, _ = commonLogger.InitLogger("app")
var dockerHubAddress = os.Getenv("IMAGE_FACTORY_HUB_ADDRESS")
var genericServiceTemplateID = os.Getenv("GENERIC_SERVICE_TEMPLATE_ID")
var catalogInstancesAndK8SCheckInternal = 60 * time.Second

const checkAfterErrorIntervalSec = 5 * time.Second

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

	interval := os.Getenv("CATALOG_INSTANCES_AND_K8S_SYNC_INTERVAL")
	if interval != "" {
		tmp, err := time.ParseDuration(interval)
		if err != nil {
			logger.Fatal("Specified interval is not a duration: " + interval)
		}
		logger.Info("Overriding default k8s and catalog sync interval to: ", tmp)
		catalogInstancesAndK8SCheckInternal = tmp
	}
	k8SAndCatalogSyncer := synchronization.GetK8SAndCatalogSynchronizer(config.CatalogApi, config.KubernetesApi)

	monitoringLoop(queueManager, k8SAndCatalogSyncer)
	waitGroup.Done()
}

func monitoringLoop(queueManager *QueueManager, k8SAndCatalogSyncer *synchronization.K8SAndCatalogSyncer) {
	catalogAndK8sSyncerTicker := time.NewTicker(catalogInstancesAndK8SCheckInternal)

	go queueManager.StartMonitoringCatalogInstances(catalogModels.WatchFromNow)
	go queueManager.StartMonitoringCatalogImages(catalogModels.WatchFromNow)

	for {
		select {
		case <-catalogAndK8sSyncerTicker.C:
			if err := k8SAndCatalogSyncer.SingleSync(); err != nil {
				logger.Error("Syncing Catalog state with K8S failed:", err)
			}
		case <-util.GetTerminationObserverChannel():
			catalogAndK8sSyncerTicker.Stop()
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

func (q *QueueManager) CheckAndApplyActionForAllImages() {
	images, _, err := config.CatalogApi.ListImages()
	if err != nil {
		logger.Error("Failed to ListImages! Actions will be not applied! error:", err)
		return
	}
	for _, image := range images {
		q.applyActionForImage(image.Id, string(image.State))
	}
}

func (q *QueueManager) CheckAndApplyActionForAllInstances() {
	instances, _, err := config.CatalogApi.ListInstances()
	if err != nil {
		logger.Error("Failed to ListInstances! Actions will be not applied! error:", err)
		return
	}
	for _, instance := range instances {
		q.applyActionForImage(instance.Id, instance.State.String())
	}
}

func (q *QueueManager) StartMonitoringCatalogImages(afterIndex uint64) {
	go q.MonitorCatalogImages(catalogModels.WatchFromNow)
	q.CheckAndApplyActionForAllInstances()
}

func (q *QueueManager) MonitorCatalogImages(afterIndex uint64) {
	monitorImagesSyncTicker := time.NewTicker(time.Millisecond)
	defer monitorImagesSyncTicker.Stop()

	for {
		select {
		case <-util.GetTerminationObserverChannel():
			return
		case <-monitorImagesSyncTicker.C:
			stateChange, _, err := config.CatalogApi.WatchImages(afterIndex)
			if err != nil {
				logger.Error("WatchImages error, start monitoring from beginning, cause: ", err)
				time.Sleep(checkAfterErrorIntervalSec)
				go q.StartMonitoringCatalogImages(catalogModels.WatchFromNow)
				return
			}
			q.applyActionForImage(stateChange.Id, stateChange.State)
			afterIndex = stateChange.Index
		}
	}
}

func (q *QueueManager) applyActionForImage(id, state string) {
	switch catalogModels.ImageState(state) {
	case catalogModels.ImageStatePending:
		q.sendMessageAndReconnectIfError(getBuildImagePostRequest(id),
			imageFactoryModels.IMAGE_FACTORY_QUEUE_NAME,
			imageFactoryModels.IMAGE_FACTORY_IMAGE_ROUTING_KEY)
	case catalogModels.ImageStateReady:
		if catalogModels.IsApplicationInstance(id) {
			err := ExecuteFlowForUserDefinedApp(id)
			if err != nil {
				logger.Errorf("Failed to create application, instance id: %s, err: %v", id, err)
			}
		}
		if catalogModels.IsUserDefinedOffering(id) {
			err := ExecuteFlowForUserDefinedOffering(id)
			if err != nil {
				logger.Errorf("Failed to create offering from binary, instance id: %s, err: %v", id, err)
			}
		}
	default:
		logger.Debugf("State is not supported by Monitor: %s", state)
	}
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
		logger.Errorf("cannot adjust template id and image - template id:", templateID)
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

	patches := []catalogModels.Patch{{
		Operation: catalogModels.OperationUpdate,
		Field:     keyNameToUpdate,
		Value:     marshaledNewStateValue,
		PrevValue: marshaledOldStateValue,
	}}
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

	patches := []catalogModels.Patch{{
		Operation: catalogModels.OperationUpdate,
		Field:     keyNameToUpdate,
		Value:     marshaledNewStateValue,
		PrevValue: marshaledOldStateValue,
	}}
	return config.CatalogApi.UpdateService(serviceId, patches)
}

func getImageAddress(id string) string {
	return dockerHubAddress + "/" + id
}

func (q *QueueManager) StartMonitoringCatalogInstances(afterIndex uint64) {
	go q.MonitorCatalogInstances(catalogModels.WatchFromNow)
	q.CheckAndApplyActionForAllInstances()
}

func (q *QueueManager) MonitorCatalogInstances(afterIndex uint64) {
	monitorInstancesSyncTicker := time.NewTicker(time.Millisecond)
	defer monitorInstancesSyncTicker.Stop()

	for {
		select {
		case <-util.GetTerminationObserverChannel():
			return
		case <-monitorInstancesSyncTicker.C:
			stateChange, _, err := config.CatalogApi.WatchInstances(afterIndex)
			if err != nil {
				logger.Error("WatchInstances error, start monitoring from beginning, cause: ", err)
				time.Sleep(checkAfterErrorIntervalSec)
				go q.StartMonitoringCatalogInstances(catalogModels.WatchFromNow)
				return
			}
			q.applyActionForInstance(stateChange.Id, stateChange.State)
			afterIndex = stateChange.Index
		}
	}
}

func (q *QueueManager) applyActionForInstance(id, state string) {
	switch catalogModels.InstanceState(state) {
	case catalogModels.InstanceStateRequested:
		instance, _, err := config.CatalogApi.GetInstance(id)
		if err != nil {
			logger.Errorf("Instance with id: %s is not going to be created! GetInstance error:", id, err)
		}
		q.sendMessageAndReconnectIfError(getCreateInstanceRequest(instance),
			containerBrokerModels.CONTAINER_BROKER_QUEUE_NAME,
			containerBrokerModels.CONTAINER_BROKER_CREATE_ROUTING_KEY)
	case catalogModels.InstanceStateDestroyReq:
		q.sendMessageAndReconnectIfError(getDeleteInstanceRequest(id),
			containerBrokerModels.CONTAINER_BROKER_QUEUE_NAME,
			containerBrokerModels.CONTAINER_BROKER_DELETE_ROUTING_KEY)
	case catalogModels.InstanceStateStartReq, catalogModels.InstanceStateStopReq,
		catalogModels.InstanceStateReconfiguration:
		q.sendMessageAndReconnectIfError(getScaleInstanceRequest(id),
			containerBrokerModels.CONTAINER_BROKER_QUEUE_NAME,
			containerBrokerModels.CONTAINER_BROKER_SCALE_ROUTING_KEY)
	default:
		logger.Debugf("State %q is not supported by Monitor", state)
	}

}

func (q *QueueManager) sendMessageAndReconnectIfError(request interface{}, queueName, routingKey string) {
	message, err := json.Marshal(request)
	if err != nil {
		logger.Errorf("Failed to prepare request for queue: %s, routingKey: %s", queueName, routingKey)
		return
	}

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
