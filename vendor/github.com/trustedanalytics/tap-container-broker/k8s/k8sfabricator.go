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

package k8s

import (
	"errors"
	"fmt"
	"time"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/apis/batch"
	"k8s.io/kubernetes/pkg/apis/extensions"
	clientK8s "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/util/sets"
	"k8s.io/kubernetes/pkg/util/wait"

	"github.com/trustedanalytics/tap-ceph-broker/client"
	"github.com/trustedanalytics/tap-container-broker/models"
	"github.com/trustedanalytics/tap-template-repository/model"
)

const (
	NotFound string = "not found"
)

type KubernetesApi interface {
	FabricateService(instanceId string, parameters map[string]string, component *model.KubernetesComponent) error
	GetFabricatedServicesForAllOrgs() ([]*model.KubernetesComponent, error)
	GetFabricatedServices(organization string) ([]*model.KubernetesComponent, error)
	DeleteAllByInstanceId(instanceId string) error
	DeleteAllPersistentVolumeClaims() error
	GetAllPersistentVolumes() ([]api.PersistentVolume, error)
	GetDeploymentsEnvsByInstanceId(instanceId string) ([]models.ContainerCredenials, error)
	GetService(instanceId string) ([]api.Service, error)
	GetServices() ([]api.Service, error)
	GetPodsStateByInstanceId(instanceId string) ([]PodStatus, error)
	GetPodsStateForAllServices() (map[string][]PodStatus, error)
	ListDeployments() (*extensions.DeploymentList, error)
	CreateConfigMap(configMap *api.ConfigMap) error
	GetConfigMap(name string) (*api.ConfigMap, error)
	GetSecret(name string) (*api.Secret, error)
	CreateSecret(secret api.Secret) error
	DeleteSecret(name string) error
	UpdateSecret(secret api.Secret) error
	GetJobs() (*batch.JobList, error)
	GetJobsByInstanceId(instanceId string) (*batch.JobList, error)
	DeleteJob(jobName string) error
	GetPod(name string) (*api.Pod, error)
	CreatePod(pod api.Pod) error
	DeletePod(podName string) error
	GetSpecificPodLogs(pod api.Pod) (map[string]string, error)
	GetPodsLogs(instanceId string) (map[string]string, error)
	GetJobLogs(job batch.Job) (map[string]string, error)
	CreateJob(job *batch.Job, instanceId string) error
	ScaleDeploymentAndWait(deployment *extensions.Deployment, replicas int) error
	UpdateDeployment(deployment *extensions.Deployment) (*extensions.Deployment, error)
	GetDeployment(name string) (*extensions.Deployment, error)
	GetIngressHosts(instanceId string) ([]string, error)
}

type K8Fabricator struct {
	client           KubernetesClient
	extensionsClient ExtensionsInterface
	cephClient       client.CephBroker
}

func GetNewK8FabricatorInstance(creds K8sClusterCredentials, cephClient client.CephBroker) (*K8Fabricator, error) {
	result := K8Fabricator{}
	client, err := GetNewClient(creds)
	if err != nil {
		return &result, err
	}

	extensionClient, err := GetNewExtensionsClient(creds)
	if err != nil {
		return &result, err
	}

	result.client = client
	result.extensionsClient = extensionClient
	result.cephClient = cephClient
	return &result, err
}

const InstanceIdLabel string = "instance_id"
const managedByLabel string = "managed_by"
const jobName string = "job-name"
const defaultCephImageSizeMB = 200

func (k *K8Fabricator) FabricateService(instanceId string, parameters map[string]string, component *model.KubernetesComponent) error {
	extraEnvironments := []api.EnvVar{{Name: "TAP_K8S", Value: "true"}}
	for key, value := range parameters {
		extraUserParam := api.EnvVar{
			Name:  ConvertToProperEnvName(key),
			Value: value,
		}
		extraEnvironments = append(extraEnvironments, extraUserParam)
	}
	logger.Debugf("Intance: %s extra parameters value: %v", instanceId, extraEnvironments)

	for _, sc := range component.Secrets {
		if _, err := k.client.Secrets(api.NamespaceDefault).Create(sc); err != nil {
			return err
		}
	}

	for _, claim := range component.PersistentVolumeClaims {
		if _, err := k.client.PersistentVolumeClaims(api.NamespaceDefault).Create(claim); err != nil {
			return err
		}
	}

	for _, svc := range component.Services {
		if _, err := k.client.Services(api.NamespaceDefault).Create(svc); err != nil {
			return err
		}
	}

	for _, acc := range component.ServiceAccounts {
		if _, err := k.client.ServiceAccounts(api.NamespaceDefault).Create(acc); err != nil {
			return err
		}
	}

	for _, deployment := range component.Deployments {
		for i, container := range deployment.Spec.Template.Spec.Containers {
			deployment.Spec.Template.Spec.Containers[i].Env = append(container.Env, extraEnvironments...)
		}

		if err := processDeploymentVolumes(*deployment, k.cephClient, true); err != nil {
			return err
		}

		if _, err := k.extensionsClient.Deployments(api.NamespaceDefault).Create(deployment); err != nil {
			return err
		}
	}

	for _, ing := range component.Ingresses {
		if _, err := k.extensionsClient.Ingress(api.NamespaceDefault).Create(ing); err != nil {
			return err
		}
	}

	return nil
}

func (k *K8Fabricator) GetFabricatedServicesForAllOrgs() ([]*model.KubernetesComponent, error) {
	// TODO: iterate over all organizations here
	return k.GetFabricatedServices(api.NamespaceDefault)
}

func (k *K8Fabricator) GetFabricatedServices(organization string) ([]*model.KubernetesComponent, error) {
	selector, err := getSelectorForManagedByLabel()
	if err != nil {
		return nil, err
	}
	listOptions := api.ListOptions{
		LabelSelector: selector,
	}

	pvcs, err := k.client.PersistentVolumeClaims(organization).List(listOptions)
	if err != nil {
		return nil, err
	}
	deployments, err := k.extensionsClient.Deployments(organization).List(listOptions)
	if err != nil {
		return nil, err
	}
	ings, err := k.extensionsClient.Ingress(organization).List(listOptions)
	if err != nil {
		return nil, err
	}
	svcs, err := k.client.Services(organization).List(listOptions)
	if err != nil {
		return nil, err
	}
	sAccounts, err := k.client.ServiceAccounts(organization).List(listOptions)
	if err != nil {
		return nil, err
	}
	secrets, err := k.client.Secrets(organization).List(listOptions)
	if err != nil {
		return nil, err
	}
	return groupIntoComponents(
		pvcs.Items,
		deployments.Items,
		ings.Items,
		svcs.Items,
		sAccounts.Items,
		secrets.Items), nil
}

func groupIntoComponents(
	pvcs []api.PersistentVolumeClaim,
	deployments []extensions.Deployment,
	ings []extensions.Ingress,
	svcs []api.Service,
	sAccounts []api.ServiceAccount,
	secrets []api.Secret,
) []*model.KubernetesComponent {

	components := make(map[string]*model.KubernetesComponent)

	for _, pvc := range pvcs {
		id := pvc.Labels[InstanceIdLabel]
		component := ensureAndGetKubernetesComponent(components, id)
		component.PersistentVolumeClaims = append(component.PersistentVolumeClaims, &pvc)
	}

	for _, deployment := range deployments {
		id := deployment.Labels[InstanceIdLabel]
		component := ensureAndGetKubernetesComponent(components, id)
		component.Deployments = append(component.Deployments, &deployment)
	}

	for _, ing := range ings {
		id := ing.Labels[InstanceIdLabel]
		component := ensureAndGetKubernetesComponent(components, id)
		component.Ingresses = append(component.Ingresses, &ing)
	}

	for _, svc := range svcs {
		id := svc.Labels[InstanceIdLabel]
		component := ensureAndGetKubernetesComponent(components, id)
		component.Services = append(component.Services, &svc)
	}

	for _, account := range sAccounts {
		id := account.Labels[InstanceIdLabel]
		component := ensureAndGetKubernetesComponent(components, id)
		component.ServiceAccounts = append(component.ServiceAccounts, &account)
	}

	for _, secret := range secrets {
		id := secret.Labels[InstanceIdLabel]
		component := ensureAndGetKubernetesComponent(components, id)
		component.Secrets = append(component.Secrets, &secret)
	}

	var list []*model.KubernetesComponent
	for _, component := range components {
		list = append(list, component)
	}
	return list
}

func ensureAndGetKubernetesComponent(components map[string]*model.KubernetesComponent,
	id string) *model.KubernetesComponent {

	component, found := components[id]
	if !found {
		component = &model.KubernetesComponent{}
		components[id] = component
	}
	return component
}

func (k *K8Fabricator) ScaleDeploymentAndWait(deployment *extensions.Deployment, replicas int) error {
	deployment.Spec.Replicas = int32(replicas)
	if _, err := k.extensionsClient.Deployments(api.NamespaceDefault).Update(deployment); err != nil {
		return err
	}
	return wait.PollInfinite(time.Millisecond*100, clientK8s.DeploymentHasDesiredReplicas(k.extensionsClient, deployment))
}

func (k *K8Fabricator) UpdateDeployment(deployment *extensions.Deployment) (*extensions.Deployment, error) {
	return k.extensionsClient.Deployments(api.NamespaceDefault).Update(deployment)
}

func (k *K8Fabricator) GetDeployment(name string) (*extensions.Deployment, error) {
	return k.extensionsClient.Deployments(api.NamespaceDefault).Get(name)
}

func (k *K8Fabricator) CreateJob(job *batch.Job, instanceId string) error {
	logger.Debug("Creating Job. InstanceId:", instanceId)
	_, err := k.extensionsClient.Jobs(api.NamespaceDefault).Create(job)
	return err
}

func (k *K8Fabricator) GetJobs() (*batch.JobList, error) {
	selector, err := getSelectorForManagedByLabel()
	if err != nil {
		return nil, err
	}

	return k.extensionsClient.Jobs(api.NamespaceDefault).List(api.ListOptions{
		LabelSelector: selector,
	})
}

func (k *K8Fabricator) GetJobsByInstanceId(instanceId string) (*batch.JobList, error) {
	selector, err := getSelectorForInstanceIdLabel(instanceId)
	if err != nil {
		return nil, err
	}

	return k.extensionsClient.Jobs(api.NamespaceDefault).List(api.ListOptions{
		LabelSelector: selector,
	})
}

func (k *K8Fabricator) DeleteJob(jobName string) error {
	return k.extensionsClient.Jobs(api.NamespaceDefault).Delete(jobName, &api.DeleteOptions{})
}

func (k *K8Fabricator) GetJobLogs(job batch.Job) (map[string]string, error) {
	jobPodsSelector, err := getSelectorBySpecificJob(job)
	if err != nil {
		return nil, err
	}
	return k.getLogs(jobPodsSelector)
}

func (k *K8Fabricator) GetSpecificPodLogs(pod api.Pod) (map[string]string, error) {
	result := make(map[string]string)
	for _, container := range pod.Spec.Containers {
		byteBody, err := k.client.Pods(api.NamespaceDefault).GetLogs(pod.Name, &api.PodLogOptions{Container: container.Name}).Do().Raw()
		if err != nil {
			logger.Error(fmt.Sprintf("Can't get logs for pod: %s and container: %s", pod.Name, container.Name))
			return result, err
		}
		result[pod.Name+"-"+container.Name] = string(byteBody)
	}
	return result, nil
}

func (k *K8Fabricator) GetPodsLogs(instanceId string) (map[string]string, error) {
	podsSelector, err := getSelectorForInstanceIdLabel(instanceId)
	if err != nil {
		return nil, err
	}
	return k.getLogs(podsSelector)
}

func (k *K8Fabricator) getLogs(selector labels.Selector) (map[string]string, error) {
	result := make(map[string]string)

	pods, err := k.client.Pods(api.NamespaceDefault).List(api.ListOptions{
		LabelSelector: selector,
	})
	if err != nil {
		return nil, err
	}

	if len(pods.Items) == 0 {
		return result, nil
	}

	isThereAtLeastOneSuccessGetLogTry := false
	for _, pod := range pods.Items {
		for _, container := range pod.Spec.Containers {
			byteBody, err := k.client.Pods(api.NamespaceDefault).GetLogs(pod.Name, &api.PodLogOptions{
				Container: container.Name}).Do().Raw()
			if err != nil {
				logger.Error(fmt.Sprintf("Can't get logs for pod: %s and container: %s", pod.Name, container.Name))
			} else {
				isThereAtLeastOneSuccessGetLogTry = true
			}
			result[pod.Name+"-"+container.Name] = string(byteBody)
		}
	}
	if isThereAtLeastOneSuccessGetLogTry {
		return result, nil
	} else {
		return result, errors.New("Can't fetch logs, err:" + err.Error())
	}
}

func (k *K8Fabricator) GetIngressHosts(instanceId string) ([]string, error) {
	result := []string{}
	ingresSelector, err := getSelectorForInstanceIdLabel(instanceId)
	if err != nil {
		return nil, err
	}

	ingresses, err := k.extensionsClient.Ingress(api.NamespaceDefault).List(api.ListOptions{
		LabelSelector: ingresSelector,
	})
	if err != nil {
		return nil, err
	}

	for _, ingress := range ingresses.Items {
		for _, rule := range ingress.Spec.Rules {
			result = append(result, rule.Host)
		}
	}
	return result, err
}

func (k *K8Fabricator) DeleteAllByInstanceId(instanceId string) error {
	selector, err := getSelectorForInstanceIdLabel(instanceId)
	if err != nil {
		return err
	}

	accs, err := k.client.ServiceAccounts(api.NamespaceDefault).List(api.ListOptions{
		LabelSelector: selector,
	})
	if err != nil {
		logger.Error("List service accounts failed:", err)
		return err
	}
	var name string
	for _, i := range accs.Items {
		name = i.ObjectMeta.Name
		logger.Debug("Delete service account:", name)
		err = k.client.ServiceAccounts(api.NamespaceDefault).Delete(name)
		if err != nil {
			logger.Error("Delete service account failed:", err)
			return err
		}
	}

	svcs, err := k.client.Services(api.NamespaceDefault).List(api.ListOptions{
		LabelSelector: selector,
	})
	if err != nil {
		logger.Error("List services failed:", err)
		return err
	}
	for _, i := range svcs.Items {
		name = i.ObjectMeta.Name
		logger.Debug("Delete service:", name)
		err = k.client.Services(api.NamespaceDefault).Delete(name)
		if err != nil {
			logger.Error("Delete service failed:", err)
			return err
		}
	}

	ings, err := k.extensionsClient.Ingress(api.NamespaceDefault).List(api.ListOptions{
		LabelSelector: selector,
	})
	if err != nil {
		logger.Error("List ingresses failed:", err)
		return err
	}
	for _, i := range ings.Items {
		name = i.ObjectMeta.Name
		logger.Debug("Delete ingress:", name)
		err = k.extensionsClient.Ingress(api.NamespaceDefault).Delete(name, &api.DeleteOptions{})
		if err != nil {
			logger.Error("Delete ingress failed:", err)
			return err
		}
	}

	if err = NewDeploymentControllerManager(k.extensionsClient, k.cephClient).DeleteAll(selector); err != nil {
		logger.Error("Delete deployment failed:", err)
		return err
	}

	secrets, err := k.client.Secrets(api.NamespaceDefault).List(api.ListOptions{
		LabelSelector: selector,
	})
	if err != nil {
		logger.Error("List secrets failed:", err)
		return err
	}
	for _, i := range secrets.Items {
		name = i.ObjectMeta.Name
		logger.Debug("Delete secret:", name)
		err = k.client.Secrets(api.NamespaceDefault).Delete(name)
		if err != nil {
			logger.Error("Delete secret failed:", err)
			return err
		}
	}

	pvcs, err := k.client.PersistentVolumeClaims(api.NamespaceDefault).List(api.ListOptions{
		LabelSelector: selector,
	})
	if err != nil {
		logger.Error("List PersistentVolumeClaims failed:", err)
		return err
	}
	for _, i := range pvcs.Items {
		name = i.ObjectMeta.Name
		logger.Debug("Delete PersistentVolumeClaims:", name)
		err = k.client.PersistentVolumeClaims(api.NamespaceDefault).Delete(name)
		if err != nil {
			logger.Error("Delete PersistentVolumeClaims failed:", err)
			return err
		}
	}
	return nil
}

func (k *K8Fabricator) DeleteAllPersistentVolumeClaims() error {
	pvList, err := k.client.PersistentVolumeClaims(api.NamespaceDefault).List(api.ListOptions{
		LabelSelector: labels.NewSelector(),
	})
	if err != nil {
		logger.Error("List PersistentVolumeClaims failed:", err)
		return err
	}

	var errorFound bool = false
	for _, i := range pvList.Items {
		name := i.ObjectMeta.Name
		logger.Debug("Delete PersistentVolumeClaims:", name)
		err = k.client.PersistentVolumeClaims(api.NamespaceDefault).Delete(name)
		if err != nil {
			logger.Error("Delete PersistentVolumeClaims: "+name+" failed!", err)
			errorFound = true
		}
	}

	if errorFound {
		return errors.New("Error on deleting PersistentVolumeClaims!")
	} else {
		return nil
	}
}

func (k *K8Fabricator) GetAllPersistentVolumes() ([]api.PersistentVolume, error) {
	pvList, err := k.client.PersistentVolumes().List(api.ListOptions{
		LabelSelector: labels.NewSelector(),
	})
	if err != nil {
		logger.Error("List PersistentVolume failed:", err)
		return nil, err
	}
	return pvList.Items, nil
}

func (k *K8Fabricator) GetService(instanceId string) ([]api.Service, error) {
	response := []api.Service{}

	selector, err := getSelectorForInstanceIdLabel(instanceId)
	if err != nil {
		return response, err
	}

	serviceList, err := k.client.Services(api.NamespaceDefault).List(api.ListOptions{
		LabelSelector: selector,
	})
	if err != nil {
		logger.Error("ListServices failed:", err)
		return response, err
	}

	return serviceList.Items, nil
}

func (k *K8Fabricator) GetServices() ([]api.Service, error) {
	response := []api.Service{}

	selector, err := getSelectorForManagedByLabel()
	if err != nil {
		logger.Error("GetSelectorForManagedByLabel error", err)
		return response, err
	}

	serviceList, err := k.client.Services(api.NamespaceDefault).List(api.ListOptions{
		LabelSelector: selector,
	})
	if err != nil {
		logger.Error("ListServices failed:", err)
		return response, err
	}
	return serviceList.Items, nil
}

func (k *K8Fabricator) ListDeployments() (*extensions.DeploymentList, error) {
	selector, err := getSelectorForManagedByLabel()
	if err != nil {
		return nil, err
	}

	return NewDeploymentControllerManager(k.extensionsClient, k.cephClient).List(selector)
}

type PodStatus struct {
	PodName         string
	InstanceId      string
	Status          api.PodPhase
	StatusMessage   string
	ContainerStatus []api.ContainerStatus
}

func (k *K8Fabricator) GetPodsStateByInstanceId(instanceId string) ([]PodStatus, error) {
	result := []PodStatus{}
	selector, err := getSelectorForInstanceIdLabel(instanceId)
	if err != nil {
		return result, err
	}

	pods, err := k.client.Pods(api.NamespaceDefault).List(api.ListOptions{
		LabelSelector: selector,
	})
	if err != nil {
		return result, err
	}

	for _, pod := range pods.Items {
		podStatus := PodStatus{
			pod.Name, instanceId, pod.Status.Phase, pod.Status.Message, pod.Status.ContainerStatuses,
		}
		result = append(result, podStatus)
	}
	return result, nil
}

func (k *K8Fabricator) GetPodsStateForAllServices() (map[string][]PodStatus, error) {
	result := map[string][]PodStatus{}

	selector, err := getSelectorForManagedByLabel()
	if err != nil {
		return result, err
	}

	pods, err := k.client.Pods(api.NamespaceDefault).List(api.ListOptions{
		LabelSelector: selector,
	})
	if err != nil {
		return result, err
	}

	for _, pod := range pods.Items {
		instanceId := pod.Labels[InstanceIdLabel]
		if instanceId != "" {
			podStatus := PodStatus{
				pod.Name, instanceId, pod.Status.Phase, pod.Status.Message, pod.Status.ContainerStatuses,
			}
			result[instanceId] = append(result[instanceId], podStatus)
		}
	}
	return result, nil
}

func (k *K8Fabricator) GetPod(name string) (*api.Pod, error) {
	result, err := k.client.Pods(api.NamespaceDefault).Get(name)
	if err != nil {
		return &api.Pod{}, err
	}
	return result, nil
}

func (k *K8Fabricator) CreatePod(pod api.Pod) error {
	_, err := k.client.Pods(api.NamespaceDefault).Create(&pod)
	return err
}

func (k *K8Fabricator) DeletePod(podName string) error {
	return k.client.Pods(api.NamespaceDefault).Delete(podName, &api.DeleteOptions{})
}

type ServiceCredential struct {
	Name  string
	Host  string
	Ports []api.ServicePort
}

func (k *K8Fabricator) CreateConfigMap(configMap *api.ConfigMap) error {
	_, err := k.client.ConfigMaps(api.NamespaceDefault).Create(configMap)
	return err
}

func (k *K8Fabricator) GetConfigMap(name string) (*api.ConfigMap, error) {
	configMap, err := k.client.ConfigMaps(api.NamespaceDefault).Get(name)
	if err != nil {
		return &api.ConfigMap{}, err
	}
	return configMap, nil
}

func (k *K8Fabricator) GetSecret(name string) (*api.Secret, error) {
	result, err := k.client.Secrets(api.NamespaceDefault).Get(name)
	if err != nil {
		return &api.Secret{}, err
	}
	return result, nil
}

func (k *K8Fabricator) CreateSecret(secret api.Secret) error {
	_, err := k.client.Secrets(api.NamespaceDefault).Create(&secret)
	return err
}

func (k *K8Fabricator) DeleteSecret(name string) error {
	err := k.client.Secrets(api.NamespaceDefault).Delete(name)
	return err
}

func (k *K8Fabricator) UpdateSecret(secret api.Secret) error {
	_, err := k.client.Secrets(api.NamespaceDefault).Update(&secret)
	return err
}

func (k *K8Fabricator) GetDeploymentsEnvsByInstanceId(instanceId string) ([]models.ContainerCredenials, error) {
	result := []models.ContainerCredenials{}

	selector, err := getSelectorForInstanceIdLabel(instanceId)
	if err != nil {
		return result, err
	}

	// this selector will be use to fetch all secrets/configMaps beacuse we need to find bound envs values
	selectorAll, err := getSelectorForManagedByLabel()
	if err != nil {
		return result, err
	}

	deployments, err := NewDeploymentControllerManager(k.extensionsClient, k.cephClient).List(selector)
	if err != nil {
		return result, err
	}

	if len(deployments.Items) < 1 {
		return result, errors.New(fmt.Sprintf("Deployments associated with the service %s are not found", instanceId))
	}

	secrets, err := k.client.Secrets(api.NamespaceDefault).List(api.ListOptions{
		LabelSelector: selectorAll,
	})
	if err != nil {
		logger.Error("List secrets failed:", err)
		return result, err
	}

	configMaps, err := k.client.ConfigMaps(api.NamespaceDefault).List(api.ListOptions{
		LabelSelector: selectorAll,
	})
	if err != nil {
		logger.Error("List configMaps failed:", err)
		return result, err
	}

	for _, deployment := range deployments.Items {
		for _, container := range deployment.Spec.Template.Spec.Containers {
			containerEnvs := models.ContainerCredenials{}
			containerEnvs.Name = deployment.Name + "_" + container.Name
			containerEnvs.Envs = make(map[string]interface{})

			for _, env := range container.Env {
				if env.Value != "" {
					containerEnvs.Envs[env.Name] = env.Value
				} else if env.ValueFrom != nil && env.ValueFrom.SecretKeyRef != nil {
					containerEnvs.Envs[env.Name] = findSecretValue(secrets, env.ValueFrom.SecretKeyRef)
				} else if env.ValueFrom != nil && env.ValueFrom.ConfigMapKeyRef != nil {
					containerEnvs.Envs[env.Name] = findConfigMapValue(configMaps, env.ValueFrom.ConfigMapKeyRef)
				}
			}
			result = append(result, containerEnvs)
		}
	}
	return result, nil
}

func findSecretValue(secrets *api.SecretList, selector *api.SecretKeySelector) string {
	for _, secret := range secrets.Items {
		if secret.Name != selector.Name {
			continue
		}

		for key, value := range secret.Data {
			if key == selector.Key {
				return string((value))
			}
		}
	}
	logger.Warningf("Key %s not found in Secret", selector.Key)
	return ""
}

func findConfigMapValue(configMaps *api.ConfigMapList, selector *api.ConfigMapKeySelector) string {
	for _, configMap := range configMaps.Items {
		if configMap.Name != selector.Name {
			continue
		}

		for key, value := range configMap.Data {
			if key == selector.Key {
				return string(value)
			}
		}
	}
	logger.Warningf("Key %s not found in ConfigMap", selector.Key)
	return ""
}

func getSelectorForInstanceIdLabel(instanceId string) (labels.Selector, error) {
	selector := labels.NewSelector()
	managedByReq, err := labels.NewRequirement(managedByLabel, labels.EqualsOperator, sets.NewString("TAP"))
	if err != nil {
		return selector, err
	}
	instanceIdReq, err := labels.NewRequirement(InstanceIdLabel, labels.EqualsOperator, sets.NewString(instanceId))
	if err != nil {
		return selector, err
	}
	return selector.Add(*managedByReq, *instanceIdReq), nil
}

func getSelectorForManagedByLabel() (labels.Selector, error) {
	selector := labels.NewSelector()
	managedByReq, err := labels.NewRequirement(managedByLabel, labels.EqualsOperator, sets.NewString("TAP"))
	if err != nil {
		return selector, err
	}
	return selector.Add(*managedByReq), nil
}

func getSelectorBySpecificJob(job batch.Job) (labels.Selector, error) {
	selector := labels.NewSelector()
	for k, v := range job.Labels {
		requirement, err := labels.NewRequirement(k, labels.EqualsOperator, sets.NewString(v))
		if err != nil {
			return selector, err
		}
		selector.Add(*requirement)
	}
	jobNameReq, err := labels.NewRequirement(jobName, labels.EqualsOperator, sets.NewString(job.Name))
	if err != nil {
		return selector, err
	}
	return selector.Add(*jobNameReq), nil
}
