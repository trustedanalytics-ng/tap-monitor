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
	"os"
	"sync"
	"time"

	"github.com/streadway/amqp"

	catalogModels "github.com/trustedanalytics-ng/tap-catalog/models"
	containerBrokerModels "github.com/trustedanalytics-ng/tap-container-broker/models"
	commonLogger "github.com/trustedanalytics-ng/tap-go-common/logger"
	"github.com/trustedanalytics-ng/tap-go-common/queue"
	"github.com/trustedanalytics-ng/tap-go-common/util"
	imageFactoryModels "github.com/trustedanalytics-ng/tap-image-factory/models"
	"github.com/trustedanalytics-ng/tap-monitor/app/synchronization"
)

var logger, _ = commonLogger.InitLogger("app")
var dockerHubAddress = os.Getenv("IMAGE_FACTORY_HUB_ADDRESS")
var genericServiceTemplateID = os.Getenv("GENERIC_SERVICE_TEMPLATE_ID")
var catalogInstancesAndK8SCheckInternal = 60 * time.Second
var entityHandlerRegister = []*EntityHandler{}

const checkAfterErrorIntervalSec = 5 * time.Second

type QueueManager struct {
	*amqp.Channel
	*amqp.Connection
}

type singleEntityAction func(afterIndex uint64, q *QueueManager) (uint64, error)
type processAllEntitiesType func(q *QueueManager)

type EntityHandler struct {
	singleEntityAction
	processAllEntitiesType
}

func init() {
	entityHandlerRegister = append(entityHandlerRegister, &EntityHandler{
		singleEntityAction:     processInstanceResult,
		processAllEntitiesType: checkAndApplyActionForAllInstances,
	})
	entityHandlerRegister = append(entityHandlerRegister, &EntityHandler{
		singleEntityAction:     processImageResult,
		processAllEntitiesType: checkAndApplyActionForAllImages,
	})
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

	for _, entityHandler := range entityHandlerRegister {
		afterIndex := getLatestIndexAndProcessAllEntities(queueManager, entityHandler)
		go queueManager.monitorCatalogEntities(afterIndex, entityHandler)
	}

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

func checkAndApplyActionForAllImages(q *QueueManager) {
	images, _, err := config.CatalogApi.ListImages()
	if err != nil {
		logger.Error("Failed to ListImages! Actions will be not applied! error:", err)
		return
	}
	for _, image := range images {
		q.applyActionForImage(image.Id, string(image.State))
	}
}

func checkAndApplyActionForAllInstances(q *QueueManager) {
	instances, _, err := config.CatalogApi.ListInstances()
	if err != nil {
		logger.Error("Failed to ListInstances! Actions will be not applied! error:", err)
		return
	}
	for _, instance := range instances {
		q.applyActionForImage(instance.Id, instance.State.String())
	}
}

func (q *QueueManager) monitorCatalogEntities(afterIndex uint64, entityHandler *EntityHandler) {
	for {
		select {
		case <-util.GetTerminationObserverChannel():
			return
		default:
			updatedIndex, err := entityHandler.singleEntityAction(afterIndex, q)
			if err != nil {
				time.Sleep(checkAfterErrorIntervalSec)
				afterIndex = getLatestIndexAndProcessAllEntities(q, entityHandler)
			} else {
				afterIndex = updatedIndex
			}
		}
	}
}

func getLatestIndexAndProcessAllEntities(q *QueueManager, entityHandler *EntityHandler) uint64 {
	index, _, err := config.CatalogApi.GetLatestIndex()
	afterIndex := index.Latest
	if err != nil {
		logger.Errorf("Cannot GetLatestIndex - default value (NOW) will be used! Error: %v", err)
		afterIndex = catalogModels.WatchFromNow
	}
	entityHandler.processAllEntitiesType(q)
	return afterIndex
}

func processImageResult(afterIndex uint64, q *QueueManager) (uint64, error) {
	stateChange, _, err := config.CatalogApi.WatchImages(afterIndex)
	if err != nil {
		logger.Error("WatchImages error, start monitoring from beginning, cause: ", err)
		return afterIndex, err
	}
	q.applyActionForImage(stateChange.Id, stateChange.State)
	return stateChange.Index, nil
}

func processInstanceResult(afterIndex uint64, q *QueueManager) (uint64, error) {
	stateChange, _, err := config.CatalogApi.WatchInstances(afterIndex)
	if err != nil {
		logger.Error("WatchInstances error, start monitoring from beginning, cause: ", err)
		return afterIndex, err
	}
	q.applyActionForInstance(stateChange.Id, stateChange.State)
	return stateChange.Index, nil
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

func (q *QueueManager) applyActionForInstance(id, state string) {
	switch catalogModels.InstanceState(state) {
	case catalogModels.InstanceStateRequested:
		instance, _, err := config.CatalogApi.GetInstance(id)
		if err != nil {
			logger.Errorf("Instance with id: %s is not going to be created! GetInstance error: %v", id, err)
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
