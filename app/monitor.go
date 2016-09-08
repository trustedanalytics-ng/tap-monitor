package app

import (
	"fmt"
	"sync"
	"time"

	"github.com/streadway/amqp"

	catalogModels "github.com/trustedanalytics/tap-catalog/models"
	containerBrokerModels "github.com/trustedanalytics/tap-container-broker/models"
	"github.com/trustedanalytics/tap-go-common/logger"
	"github.com/trustedanalytics/tap-go-common/queue"
	"github.com/trustedanalytics/tap-go-common/util"
)

var logger = logger_wrapper.InitLogger("app")

const CHECK_INTERVAL_SECONDS = 5

// todo this is temporary -> DPNG-10694
var createInstanceQueue = []string{}
var deleteInstanceQueue = []string{}

func StartMonitor(waitGroup *sync.WaitGroup) {
	waitGroup.Add(1)
	channel, conn := queue.GetConnectionChannel()
	queue.CreateExchangeWithQueueByRoutingKeys(channel, containerBrokerModels.CONTAINER_BROKER_QUEUE_NAME,
		[]string{containerBrokerModels.CONTAINER_BROKER_CREATE_ROUTING_KEY, containerBrokerModels.CONTAINER_BROKER_DELETE_ROUTING_KEY})

	isRunning := true
	go func() {
		<-util.GetTerminationObserverChannel()
		isRunning = false
	}()

	for isRunning {
		if err := CheckCatalogRequestedService(channel); err != nil {
			logger.Error("Proccessing Catalog data error:", err)
		}
		time.Sleep(CHECK_INTERVAL_SECONDS * time.Second)
	}

	defer conn.Close()
	defer channel.Close()
	logger.Info("Monitoring stopped")
	waitGroup.Done()
}

//todo what we should do in error case: log error and continue or break/fail
//todo should we change instance state to FAILURE on error case?
func CheckCatalogRequestedService(channel *amqp.Channel) error {
	instances, _, err := config.CatalogApi.ListInstances()
	if err != nil {
		return err
	}

	for _, instance := range instances {
		if instance.State == catalogModels.InstanceStateRequested {
			if isInstanceIdInArray(createInstanceQueue, instance.Id) {
				continue
			}
			go handleCreateInstanceRequest(instance, channel)
			createInstanceQueue = append(createInstanceQueue, instance.Id)
		} else if instance.State == catalogModels.InstanceStateDestroyReq {
			if isInstanceIdInArray(deleteInstanceQueue, instance.Id) {
				continue
			}
			deleteInstanceQueue = append(deleteInstanceQueue, instance.Id)
			go sendMessageOnQueue(instance, channel, containerBrokerModels.CONTAINER_BROKER_DELETE_ROUTING_KEY)
		}
	}
	return nil
}

// todo this needs to be move to api-console CreateApplication endpoint
func handleCreateInstanceRequest(instance catalogModels.Instance, channel *amqp.Channel) {
	//todo we need to verify for every service-broker isjntance if service-broker instance is ready already!
	if instance.Type == catalogModels.InstanceTypeApplication {
		for {
			app, _, err := config.CatalogApi.GetApplication(instance.ClassId)
			if err != nil {
				logger.Error(fmt.Sprintf("Can't process instanceId: %s - GetApplication error", instance.Id), err)
				return
			}

			// todo we should use long poll here
			image, _, err := config.CatalogApi.GetImage(app.ImageId)
			if err != nil {
				logger.Error(fmt.Sprintf("Can't process instanceId: %s - GetImage error", instance.Id), err)
				return
			} else if image.State == catalogModels.ImageStateError {
				logger.Error(fmt.Sprintf("Can't process instanceId: %s - assigned image has wrong state: %s", instance.Id, image.State))
				return
			} else if image.State == catalogModels.ImageStateReady {
				break
			} else {
				logger.Debug(fmt.Sprintf("InstanceId: %s - waiting for image to be ready. Image state: %s", instance.Id, image.State))
				time.Sleep(CHECK_INTERVAL_SECONDS * time.Second)
				continue
			}
		}
	}
	sendMessageOnQueue(instance, channel, containerBrokerModels.CONTAINER_BROKER_CREATE_ROUTING_KEY)
}

func sendMessageOnQueue(instance catalogModels.Instance, channel *amqp.Channel, routingKey string) {
	var queueMessage []byte
	var err error

	switch routingKey {
	case containerBrokerModels.CONTAINER_BROKER_CREATE_ROUTING_KEY:
		queueMessage, err = prepareCreateInstanceRequest(instance)
		if err != nil {
			logger.Error("Failed to prepare CreateInstanceRequest for instance: ", instance.Id, err)
			return
		}
	case containerBrokerModels.CONTAINER_BROKER_DELETE_ROUTING_KEY:
		queueMessage, err = prepareDeleteRequest(instance)
		if err != nil {
			logger.Error("Failed to prepare DeleteRequest for instance: ", instance.Id, err)
			return
		}
	default:
		logger.Error(fmt.Sprintf("InstanceId: %s- following routing key is not supported: %s", instance.Id, routingKey))
		return
	}

	if err := queue.SendMessageToQueue(channel, queueMessage, containerBrokerModels.CONTAINER_BROKER_QUEUE_NAME, routingKey); err != nil {
		logger.Error("Can't send message on queue! InstanceId:", instance.Id, err)
		return
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
