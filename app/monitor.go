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
	"fmt"
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

const CHECK_INTERVAL_SECONDS = 5

// todo this is temporary -> DPNG-10694
var createInstanceQueue = []string{}
var deleteInstanceQueue = []string{}
var pendingImageQueue = []string{}
var readyImageQueue = []string{}

type QueueManager struct {
	*amqp.Channel
	*amqp.Connection
}

func StartMonitor(waitGroup *sync.WaitGroup) {
	waitGroup.Add(1)

	channel, conn := setupQueueConnection()
	queueManager := &QueueManager{channel, conn}

	isRunning := true
	go func() {
		<-util.GetTerminationObserverChannel()
		isRunning = false
	}()

	for isRunning {
		if err := queueManager.CheckCatalogRequestedInstances(); err != nil {
			logger.Error("Proccessing Catalog instances error:", err)
		}
		if err := queueManager.CheckCatalogRequestedImages(); err != nil {
			logger.Error("Proccessing Catalog images error:", err)
		}
		time.Sleep(CHECK_INTERVAL_SECONDS * time.Second)
	}

	defer queueManager.Connection.Close()
	defer queueManager.Channel.Close()
	logger.Info("Monitoring stopped")
	waitGroup.Done()
}

func setupQueueConnection() (*amqp.Channel, *amqp.Connection) {
	channel, conn := queue.GetConnectionChannel()
	queue.CreateExchangeWithQueueByRoutingKeys(channel, containerBrokerModels.CONTAINER_BROKER_QUEUE_NAME,
		[]string{containerBrokerModels.CONTAINER_BROKER_CREATE_ROUTING_KEY, containerBrokerModels.CONTAINER_BROKER_DELETE_ROUTING_KEY})
	queue.CreateExchangeWithQueueByRoutingKeys(channel, imageFactoryModels.IMAGE_FACTORY_QUEUE_NAME, []string{imageFactoryModels.IMAGE_FACTORY_IMAGE_ROUTING_KEY})
	return channel, conn
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
			pendingImageQueue = append(pendingImageQueue, image.Id)
			q.sendMessageOnQueue(image, imageFactoryModels.IMAGE_FACTORY_QUEUE_NAME, imageFactoryModels.IMAGE_FACTORY_IMAGE_ROUTING_KEY)
		case catalogModels.ImageStateReady:
			if isInstanceIdInArray(readyImageQueue, image.Id) {
				continue
			}
			readyImageQueue = append(readyImageQueue, image.Id)

			if catalogModels.IsApplicationInstance(image.Id) {
				applicationId := catalogModels.GetApplicationId(image.Id)
				instances, _, err := config.CatalogApi.ListApplicationInstances(applicationId)
				if err != nil {
					logger.Error("Failed to call ListApplicationInstances for image: ", image.Id, err)
					break
				}

				if len(instances) == 0 {
					application, _, err := config.CatalogApi.GetApplication(applicationId)
					if err != nil {
						logger.Error("Failed to call GetApplication for image: ", image.Id, err)
						break
					}

					instance := catalogModels.Instance{
						Name:    application.Name,
						Type:    catalogModels.InstanceTypeApplication,
						ClassId: application.Id,
						Metadata: []catalogModels.Metadata{
							{Id: catalogModels.APPLICATION_IMAGE_ADDRESS, Value: getImageAddress(image.Id)},
						},
					}

					if _, _, err = config.CatalogApi.AddApplicationInstance(application.Id, instance); err != nil {
						logger.Error("Failed to call AddApplicationInstance for image: ", image.Id, err)
						break
					}
				}
			}
		}
	}
	return nil
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
			q.sendMessageOnQueue(instance, containerBrokerModels.CONTAINER_BROKER_QUEUE_NAME, containerBrokerModels.CONTAINER_BROKER_CREATE_ROUTING_KEY)
			createInstanceQueue = append(createInstanceQueue, instance.Id)
		} else if instance.State == catalogModels.InstanceStateDestroyReq {
			if isInstanceIdInArray(deleteInstanceQueue, instance.Id) {
				continue
			}
			deleteInstanceQueue = append(deleteInstanceQueue, instance.Id)
			q.sendMessageOnQueue(instance, containerBrokerModels.CONTAINER_BROKER_QUEUE_NAME, containerBrokerModels.CONTAINER_BROKER_DELETE_ROUTING_KEY)
		}
	}
	return nil
}

func (q *QueueManager) sendMessageOnQueue(entity interface{}, queueName, routingKey string) {
	var queueMessage []byte
	var err error

	switch routingKey {
	case imageFactoryModels.IMAGE_FACTORY_IMAGE_ROUTING_KEY:
		image, _ := entity.(catalogModels.Image)
		queueMessage, err = prepareBuildImageRequest(image)
		if err != nil {
			logger.Error("Failed to prepare BuildImagePostRequest for image: ", image.Id, err)
			break
		}

	case containerBrokerModels.CONTAINER_BROKER_CREATE_ROUTING_KEY:
		instance, _ := entity.(catalogModels.Instance)
		queueMessage, err = prepareCreateInstanceRequest(instance)
		if err != nil {
			logger.Error("Failed to prepare CreateInstanceRequest for instance: ", instance.Id, err)
			return
		}
	case containerBrokerModels.CONTAINER_BROKER_DELETE_ROUTING_KEY:
		instance, _ := entity.(catalogModels.Instance)
		queueMessage, err = prepareDeleteInstanceRequest(instance)
		if err != nil {
			logger.Error("Failed to prepare DeleteRequest for instance: ", instance.Id, err)
			return
		}
	default:
		logger.Error(fmt.Sprintf("Following routing key is not supported: %s", routingKey))
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
