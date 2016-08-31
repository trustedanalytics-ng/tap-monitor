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
	"encoding/json"
	"errors"
	"strings"
	"time"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/apis/extensions"
	clientK8s "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/util/sets"
	"k8s.io/kubernetes/pkg/util/wait"

	"github.com/trustedanalytics/tap-template-repository/model"
)

type KubernetesApi interface {
	FabricateService(instanceId, parameters string, component *model.KubernetesComponent) error
	DeleteAllByInstanceId(instanceId string) error
	DeleteAllPersistentVolumeClaims() error
	GetAllPersistentVolumes() ([]api.PersistentVolume, error)
	GetAllPodsEnvsByInstanceId(instanceId string) ([]PodEnvs, error)
	GetService(instanceId string) ([]api.Service, error)
	GetServices() ([]api.Service, error)
	GetPodsStateByServiceId(instanceId string) ([]PodStatus, error)
	GetPodsStateForAllServices() (map[string][]PodStatus, error)
	ListDeployments() (*extensions.DeploymentList, error)
	CreateConfigMap(configMap api.ConfigMap) error
	GetConfigMap(name string) (*api.ConfigMap, error)
	GetSecret(name string) (*api.Secret, error)
	CreateSecret(secret api.Secret) error
	DeleteSecret(name string) error
	UpdateSecret(secret api.Secret) error
	GetJobs() (*extensions.JobList, error)
	GetJobsByInstanceId(instanceId string) (*extensions.JobList, error)
	DeleteJob(jobName string) error
	GetPodsLogs(instanceId string) (map[string]string, error)
	CreateJob(job *extensions.Job, instanceId string) error
	ScaleDeploymentAndWait(deployment *extensions.Deployment, replicas int) error
	UpdateDeployment(deployment *extensions.Deployment) (*extensions.Deployment, error)
	GetDeployment(name string) (*extensions.Deployment, error)
}

type K8Fabricator struct {
	client           KubernetesClient
	extensionsClient ExtensionsInterface
}

func GetNewK8FabricatorInstance(creds K8sClusterCredentials) (*K8Fabricator, error) {
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
	return &result, err
}

type K8sServiceInfo struct {
	ServiceId string   `json:"serviceId"`
	Org       string   `json:"org"`
	Space     string   `json:"space"`
	Name      string   `json:"name"`
	TapPublic bool     `json:"tapPublic"`
	Uri       []string `json:"uri"`
}

//todo refactor it to instanceId
const ServiceIdLabel string = "service_id"
const managedByLabel string = "managed_by"

func (k *K8Fabricator) FabricateService(instanceId, parameters string, component *model.KubernetesComponent) error {
	extraEnvironments := []api.EnvVar{{Name: "TAP_K8S", Value: "true"}}
	if parameters != "" {
		extraUserParam := api.EnvVar{}
		err := json.Unmarshal([]byte(parameters), &extraUserParam)
		if err != nil {
			logger.Error("Unmarshalling extra user parameters error!", err)
			return err
		}

		if extraUserParam.Name != "" {
			// kubernetes env name validation:
			// "must be a C identifier (matching regex [A-Za-z_][A-Za-z0-9_]*): e.g. \"my_name\" or \"MyName\"","
			extraUserParam.Name = extraUserParam.Name + "_" + instanceId
			extraUserParam.Name = strings.Replace(extraUserParam.Name, "_", "__", -1) //name_1 --> name__1__instanceId
			extraUserParam.Name = strings.Replace(extraUserParam.Name, "-", "_", -1)  //name-1 --> name_1__instanceId

			extraEnvironments = append(extraEnvironments, extraUserParam)
		}
		logger.Debug("Extra parameters value:", extraEnvironments)
	}

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

	for _, deployment := range component.Deployments {
		for i, container := range deployment.Spec.Template.Spec.Containers {
			deployment.Spec.Template.Spec.Containers[i].Env = append(container.Env, extraEnvironments...)
		}

		if _, err := k.extensionsClient.Deployments(api.NamespaceDefault).Create(deployment); err != nil {
			return err
		}
	}

	for _, svc := range component.Services {
		if _, err := k.client.Services(api.NamespaceDefault).Create(svc); err != nil {
			return err
		}
	}

	for _, ing := range component.Ingresses {
		if _, err := k.extensionsClient.Ingress(api.NamespaceDefault).Create(ing); err != nil {
			return err
		}
	}

	for _, acc := range component.ServiceAccounts {
		if _, err := k.client.ServiceAccounts(api.NamespaceDefault).Create(acc); err != nil {
			return err
		}
	}
	return nil
}

func (k *K8Fabricator) ScaleDeploymentAndWait(deployment *extensions.Deployment, replicas int) error {
	deployment.Spec.Replicas = replicas
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

func (k *K8Fabricator) CreateJob(job *extensions.Job, instanceId string) error {
	logger.Debug("Creating Job. InstanceId:", instanceId)
	_, err := k.extensionsClient.Jobs(api.NamespaceDefault).Create(job)
	return err
}

func (k *K8Fabricator) GetJobs() (*extensions.JobList, error) {
	selector, err := getSelectorForManagedByLabel()
	if err != nil {
		return nil, err
	}

	return k.extensionsClient.Jobs(api.NamespaceDefault).List(api.ListOptions{
		LabelSelector: selector,
	})
}

func (k *K8Fabricator) GetJobsByInstanceId(instanceId string) (*extensions.JobList, error) {
	selector, err := getSelectorForServiceIdLabel(instanceId)
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

func (k *K8Fabricator) GetPodsLogs(instanceId string) (map[string]string, error) {
	result := map[string]string{}

	secretSelector, err := getSelectorForServiceIdLabel(instanceId)
	if err != nil {
		return nil, err
	}

	pods, err := k.client.Pods(api.NamespaceDefault).List(api.ListOptions{
		LabelSelector: secretSelector,
	})
	if err != nil {
		return nil, err
	}

	for _, pod := range pods.Items {
		for _, container := range pod.Spec.Containers {
			byteBody, err := k.client.Pods(api.NamespaceDefault).GetLogs(pod.Name, &api.PodLogOptions{
				Container: container.Name}).Do().Raw()
			if err != nil {
				return nil, err
			}
			result[pod.Name+"-"+container.Name] = string(byteBody)
		}
	}
	return result, nil
}

func (k *K8Fabricator) DeleteAllByInstanceId(instanceId string) error {
	selector, err := getSelectorForServiceIdLabel(instanceId)
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

	if err = NewDeploymentControllerManager(k.extensionsClient).DeleteAll(selector); err != nil {
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

	selector, err := getSelectorForServiceIdLabel(instanceId)
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

	return NewDeploymentControllerManager(k.extensionsClient).List(selector)
}

type PodStatus struct {
	PodName       string
	ServiceId     string
	Status        api.PodPhase
	StatusMessage string
}

func (k *K8Fabricator) GetPodsStateByServiceId(instanceId string) ([]PodStatus, error) {
	result := []PodStatus{}
	selector, err := getSelectorForServiceIdLabel(instanceId)
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
			pod.Name, instanceId, pod.Status.Phase, pod.Status.Message,
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
		service_id := pod.Labels[ServiceIdLabel]
		if service_id != "" {
			podStatus := PodStatus{
				pod.Name, service_id, pod.Status.Phase, pod.Status.Message,
			}
			result[service_id] = append(result[service_id], podStatus)
		}
	}
	return result, nil
}

type ServiceCredential struct {
	Name  string
	Host  string
	Ports []api.ServicePort
}

func (k *K8Fabricator) CreateConfigMap(configMap api.ConfigMap) error {
	_, err := k.client.ConfigMaps(api.NamespaceDefault).Create(&configMap)
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

type PodEnvs struct {
	RcName     string
	Containers []ContainerSimple
}

type ContainerSimple struct {
	Name string
	Envs map[string]string
}

func (k *K8Fabricator) GetAllPodsEnvsByInstanceId(instanceId string) ([]PodEnvs, error) {
	result := []PodEnvs{}

	selector, err := getSelectorForServiceIdLabel(instanceId)
	if err != nil {
		return result, err
	}

	deployments, err := NewDeploymentControllerManager(k.extensionsClient).List(selector)
	if err != nil {
		return result, err
	}

	if len(deployments.Items) < 1 {
		return result, errors.New("No deployments associated with the service: " + instanceId)
	}

	secrets, err := k.client.Secrets(api.NamespaceDefault).List(api.ListOptions{
		LabelSelector: selector,
	})
	if err != nil {
		logger.Error("List secrets failed:", err)
		return result, err
	}

	for _, deployment := range deployments.Items {
		pod := PodEnvs{}
		pod.RcName = deployment.Name
		pod.Containers = []ContainerSimple{}

		for _, container := range deployment.Spec.Template.Spec.Containers {
			simpelContainer := ContainerSimple{}
			simpelContainer.Name = container.Name
			simpelContainer.Envs = map[string]string{}

			for _, env := range container.Env {
				if env.Value == "" {
					logger.Debug("Empty env value, searching env variable in secrets")
					simpelContainer.Envs[env.Name] = findSecretValue(secrets, envNameToSecretKey(env.Name))
				} else {
					simpelContainer.Envs[env.Name] = env.Value
				}

			}
			pod.Containers = append(pod.Containers, simpelContainer)
		}
		result = append(result, pod)
	}
	return result, nil
}

func envNameToSecretKey(env_name string) string {
	lower_case_string := strings.ToLower(env_name)
	return strings.Replace(lower_case_string, "_", "-", -1)
}

func findSecretValue(secrets *api.SecretList, secret_key string) string {
	for _, i := range secrets.Items {
		for key, value := range i.Data {
			if key == secret_key {
				return string((value))
			}
		}
	}
	logger.Info("Secret key not found: ", secret_key)
	return ""
}

func getSelectorForServiceIdLabel(serviceId string) (labels.Selector, error) {
	selector := labels.NewSelector()
	managedByReq, err := labels.NewRequirement(managedByLabel, labels.EqualsOperator, sets.NewString("TAP"))
	if err != nil {
		return selector, err
	}
	serviceIdReq, err := labels.NewRequirement(ServiceIdLabel, labels.EqualsOperator, sets.NewString(serviceId))
	if err != nil {
		return selector, err
	}
	return selector.Add(*managedByReq, *serviceIdReq), nil
}

func getSelectorForManagedByLabel() (labels.Selector, error) {
	selector := labels.NewSelector()
	managedByReq, err := labels.NewRequirement(managedByLabel, labels.EqualsOperator, sets.NewString("TAP"))
	if err != nil {
		return selector, err
	}
	return selector.Add(*managedByReq), nil
}
