package app

import (
	"time"

	"github.com/streadway/amqp"

	catalogModels "github.com/trustedanalytics/tap-catalog/models"
	containerBrokerModels "github.com/trustedanalytics/tap-container-broker/models"
	"github.com/trustedanalytics/tap-go-common/logger"
	"github.com/trustedanalytics/tap-go-common/queue"
	"github.com/trustedanalytics/tap-go-common/util"
	"sync"
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
				break
			}

			queueMessage, err := prepareCreateInstanceRequest(instance)
			if err != nil {
				logger.Error("Failed to prepare Message for instance: ", instance.Id, err)
				return err
			}

			if err := queue.SendMessageToQueue(channel, queueMessage, containerBrokerModels.CONTAINER_BROKER_QUEUE_NAME,
				containerBrokerModels.CONTAINER_BROKER_CREATE_ROUTING_KEY); err != nil {
				return err
			}
			createInstanceQueue = append(createInstanceQueue, instance.Id)
		} else if instance.State == catalogModels.InstanceStateDestroyReq {
			if isInstanceIdInArray(deleteInstanceQueue, instance.Id) {
				break
			}

			queueMessage, err := prepareDeleteRequest(instance)
			if err != nil {
				logger.Error("Failed to prepare Message for instance: ", instance.Id, err)
				return err
			}

			if err := queue.SendMessageToQueue(channel, queueMessage, containerBrokerModels.CONTAINER_BROKER_QUEUE_NAME,
				containerBrokerModels.CONTAINER_BROKER_DELETE_ROUTING_KEY); err != nil {
				return err
			}
			deleteInstanceQueue = append(deleteInstanceQueue, instance.Id)
		}
	}
	return nil
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
