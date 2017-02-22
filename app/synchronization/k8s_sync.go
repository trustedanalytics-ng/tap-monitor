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

package synchronization

// equality of Catalog Instance and K8S Objects is currently done only on ID generated by TAP
// This lacks handling of case when K8S Objects are not complete (for example only one of two deployments is present on K8S).
// In such case it will tell us that the instance exists whereas it is in error state

// Possible solutions:
// Add new endpoint in template-repository to return list of available offerings templates with number of its K8S definitions

import (
	"errors"
	"time"

	"github.com/patrickmn/go-cache"

	"github.com/trustedanalytics-ng/tap-catalog/builder"
	catalogApi "github.com/trustedanalytics-ng/tap-catalog/client"
	"github.com/trustedanalytics-ng/tap-catalog/models"
	kubernetesApi "github.com/trustedanalytics-ng/tap-container-broker/k8s"
	trModels "github.com/trustedanalytics-ng/tap-template-repository/model"
)

const (
	zombieIgnoreTime = 1 * time.Hour
)

type K8SAndCatalogSyncer struct {
	catalog catalogApi.TapCatalogApi
	k8s     kubernetesApi.KubernetesApi

	zombiesCache *cache.Cache
}

func GetK8SAndCatalogSynchronizer(catalog catalogApi.TapCatalogApi, k8s kubernetesApi.KubernetesApi) *K8SAndCatalogSyncer {
	zombiesCache := cache.New(zombieIgnoreTime, 30*time.Second)
	return &K8SAndCatalogSyncer{
		catalog:      catalog,
		k8s:          k8s,
		zombiesCache: zombiesCache,
	}
}

type K8SObject trModels.KubernetesComponent

func (ko *K8SObject) validate() error {
	id := ko.getId()
	if id == "" {
		return errors.New("No K8S element in K8SObjec")
	}
	return nil
}

func (ko *K8SObject) getId() string {
	if len(ko.Deployments) > 0 {
		return getId(ko.Deployments[0].Labels)
	}
	if len(ko.ServiceAccounts) > 0 {
		return getId(ko.ServiceAccounts[0].Labels)
	}
	if len(ko.PersistentVolumeClaims) > 0 {
		return getId(ko.PersistentVolumeClaims[0].Labels)
	}
	if len(ko.Services) > 0 {
		return getId(ko.Services[0].Labels)
	}
	if len(ko.Ingresses) > 0 {
		return getId(ko.Ingresses[0].Labels)
	}
	return ""
}

func getId(labels map[string]string) string {
	return labels[kubernetesApi.InstanceIdLabel]
}

func (ko *K8SObject) isFullyRunning() bool {
	for _, depl := range ko.Deployments {
		if depl.Spec.Replicas == 0 {
			return false
		}
	}
	return true
}

func (ko *K8SObject) isFullyStopped() bool {
	for _, depl := range ko.Deployments {
		if depl.Spec.Replicas > 0 {
			return false
		}
	}
	return true
}

func (s *K8SAndCatalogSyncer) SingleSync() (err error) {
	logger.Info("Starting syncing Catalog instances with K8S")
	defer func() {
		if err == nil {
			logger.Info("Done syncing Catalog instances with K8S")
		} else {
			logger.Error("Error while syncing Catalog instances with K8S: " + err.Error())
		}
	}()

	instancesExpectedOnK8S, err := s.getInstancesExecutableOnK8S()
	if err != nil {
		logger.Error("Error while retrieving instances in catalog")
		return
	}
	logger.Debugf("Retrieved %d Catalog instances executable on K8S", len(instancesExpectedOnK8S))

	tapK8SObjects, err := s.getK8SObjectsCreatedByTAP()
	if err != nil {
		logger.Error("Error while retrieving k8s objects")
		return
	}
	logger.Debugf("Retrieved %d TAPs objects from K8S", len(tapK8SObjects))

	// below functions could be put on a slice and iterate through it
	err = s.handlePseudoRunningCatalogInstances(instancesExpectedOnK8S, tapK8SObjects)
	if err != nil {
		logger.Error("Error creating missing catalog instances in K8S")
		return
	}
	logger.Debug("Synced catalog instnaces missing on K8S")

	err = s.handlePseudoStoppedCatalogInstances(instancesExpectedOnK8S, tapK8SObjects)
	if err != nil {
		logger.Error("Error deleting not wanted catalog instances in K8S")
		return
	}
	logger.Debug("Synced catalog instnaces not wanted on K8S")

	err = s.handleK8SZombieCatalogInstances(instancesExpectedOnK8S, tapK8SObjects)
	if err != nil {
		logger.Error("Error removing catalog zombie instances in K8S")
		return
	}
	return
}

// predicates for filtering out not related items
type InstancePredicate func(*models.Instance) bool
type K8SObjectPredicate func(*K8SObject) bool

func isK8SExecutableInstance(instance *models.Instance) bool {
	return instance.Type == models.InstanceTypeApplication ||
		instance.Type == models.InstanceTypeService
}

func (s *K8SAndCatalogSyncer) getInstancesExecutableOnK8S() ([]*models.Instance, error) {
	directInstances, _, err := s.catalog.ListInstances()
	if err != nil {
		return nil, err
	}
	instances := make([]*models.Instance, len(directInstances))
	for i := range directInstances {
		instances[i] = &directInstances[i]
	}
	return filterInstances(instances, isK8SExecutableInstance), nil
}

func (s *K8SAndCatalogSyncer) getK8SObjectsCreatedByTAP() ([]*K8SObject, error) {
	kubernetesComponents, err := s.k8s.GetFabricatedComponentsForAllOrgs()
	if err != nil {
		return nil, err
	}
	var objects []*K8SObject
	for _, kc := range kubernetesComponents {
		instance := K8SObject(*kc)
		if err := instance.validate(); err != nil {
			return nil, err
		}
		objects = append(objects, &instance)
	}
	return objects, nil
}

func findZombieK8SObjects(catalogInstances []*models.Instance,
	k8sObjects []*K8SObject) []*K8SObject {

	zombies := make(map[string]*K8SObject)
	for _, ko := range k8sObjects {
		zombies[ko.getId()] = ko
	}
	for _, ci := range catalogInstances {
		delete(zombies, ci.Id)
	}
	var zombiesList []*K8SObject
	for _, z := range zombies {
		zombiesList = append(zombiesList, z)
	}
	return zombiesList
}

func (s *K8SAndCatalogSyncer) setErrorState(catalogInstance *models.Instance, msg string) error {
	logger.Info("Marking as failed catalog instance:", catalogInstance, "with", msg)
	patches, err := builder.MakePatchesForInstanceStateAndLastStateMetadata(
		msg,
		catalogInstance.State,
		models.InstanceStateFailure,
	)
	if err != nil {
		return err
	}
	_, _, err = s.catalog.UpdateInstance(catalogInstance.Id, patches)
	return err
}

type caseWithFiltering struct {
	catalogInstancesForK8S []*models.Instance
	instanceFilter         InstancePredicate

	k8sObjects []*K8SObject
	koFilter   K8SObjectPredicate
}

type InstanceHandler func(*models.Instance) error

func (c *caseWithFiltering) handle(name string, handler InstanceHandler) error {
	instances := filterInstances(c.catalogInstancesForK8S, c.instanceFilter)
	matchedInstances := filterInstancesOnK8SObjects(instances, c.k8sObjects, c.koFilter)

	logger.Infof("Handling %s instances: %d", name, len(matchedInstances))

	for _, ci := range matchedInstances {
		logger.Debugf("Fixing: %s  - %s", ci.Id, name)
		err := handler(ci)
		if err != nil {
			return err
		}
	}
	return nil
}

// handles case: ci.state=Running and any deployments.replicas == 0
func (s *K8SAndCatalogSyncer) handlePseudoRunningCatalogInstances(
	catalogInstancesForK8S []*models.Instance, k8sObjects []*K8SObject) error {

	c := caseWithFiltering{
		catalogInstancesForK8S: catalogInstancesForK8S,
		instanceFilter:         instanceShouldRunOnK8S,
		k8sObjects:             k8sObjects,
		koFilter: func(ko *K8SObject) bool {
			return !ko.isFullyRunning()
		},
	}
	return c.handle("not fully running", func(ci *models.Instance) error {
		return s.setErrorState(ci, "Instance was already running but was stopped on K8S")
	},
	)
}

// handles case: ci.state=Stopped and any deployments.replicas > 0
func (s *K8SAndCatalogSyncer) handlePseudoStoppedCatalogInstances(
	catalogInstancesForK8S []*models.Instance, k8sObjects []*K8SObject) error {

	c := caseWithFiltering{
		catalogInstancesForK8S: catalogInstancesForK8S,
		instanceFilter:         instanceShouldBeStoppedOnK8S,
		k8sObjects:             k8sObjects,
		koFilter: func(ko *K8SObject) bool {
			return !ko.isFullyStopped()
		},
	}
	return c.handle("not fully stopped", func(ci *models.Instance) error {
		return s.setErrorState(ci, "Instance was stopped but still was running on K8S")
	})
}

// This handles case: K8S object comes from TAP but there is no corresponding catalog instance
func (s *K8SAndCatalogSyncer) handleK8SZombieCatalogInstances(
	catalogInstances []*models.Instance, k8sObjects []*K8SObject) error {

	zombieK8SObjects := findZombieK8SObjects(catalogInstances, k8sObjects)
	for _, zko := range zombieK8SObjects {
		if _, found := s.zombiesCache.Get(zko.getId()); !found {
			logger.Warning("Zombie K8SObject with id: ", zko.getId())
			s.zombiesCache.Add(zko.getId(), zko, cache.DefaultExpiration)
			// We do not act here on purpose
		}
	}
	return nil
}

func instanceShouldRunOnK8S(instance *models.Instance) bool {
	return instance.State == models.InstanceStateRunning
}

func instanceShouldBeStoppedOnK8S(instance *models.Instance) bool {
	return instance.State == models.InstanceStateStopped
}

func filterInstances(instances []*models.Instance, predicate InstancePredicate) []*models.Instance {
	var result []*models.Instance
	for _, instance := range instances {
		if predicate(instance) {
			result = append(result, instance)
		}
	}
	return result
}

func k8sObjectsToMap(k8sObjects []*K8SObject) map[string]*K8SObject {
	k8sObjectsCache := make(map[string]*K8SObject)
	for _, inst := range k8sObjects {
		k8sObjectsCache[inst.getId()] = inst
	}
	return k8sObjectsCache
}

func filterInstancesOnK8SObjects(instances []*models.Instance, k8sObjects []*K8SObject, predicate K8SObjectPredicate) []*models.Instance {
	k8sCache := k8sObjectsToMap(k8sObjects)
	return filterInstances(instances, func(instance *models.Instance) bool {
		ko, found := k8sCache[instance.Id]
		if !found {
			return false
		}
		return predicate(ko)
	})
}
