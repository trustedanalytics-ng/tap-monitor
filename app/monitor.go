package app

import (
	"time"

	catalogModels "github.com/trustedanalytics/tapng-catalog/models"
	"github.com/trustedanalytics/tapng-go-common/logger"
)

var logger = logger_wrapper.InitLogger("app")

const CHECK_INTERVAL_SECONDS = 5

func StartMonitor() error {

	instancesOnQueue := []string{}
	var err error
	for {
		instancesOnQueue, err = CheckCatalogRequestedService(instancesOnQueue)
		if err != nil {
			return err
		}
		time.Sleep(CHECK_INTERVAL_SECONDS * time.Second)
	}
}


func CheckCatalogRequestedService(instancesOnQueue []string) ([]string, error){

	var instancesinStateRequested []string

	instances, _, err := config.CatalogApi.ListServicesInstances()
	if err != nil {
		return instancesinStateRequested, err
	}

	for _, instance := range instances {

		if instance.State == catalogModels.InstanceStateRequested {

			instancesinStateRequested = append(instancesinStateRequested, instance.Id)

			if isInstanceIdInArray(instancesOnQueue, instance.Id) {
				break;
			}

			queueMessage, err := createServiceInstanceBody(instance)
			if err != nil {
				logger.Error("failed to create task for instance: ", instance.Id, "err: ", err.Error())
				return instancesinStateRequested, err
			}

			err = SendMessageToQueue(queueMessage)
			if err != nil {
				return instancesinStateRequested, err
			}
		}
	}
	return instancesinStateRequested, nil
}

func isInstanceIdInArray(instanceIds []string, wantedInstanceId string) bool{
	for _, instanceId := range instanceIds {
		if instanceId == wantedInstanceId {
			return true
		}
	}
	return false
}