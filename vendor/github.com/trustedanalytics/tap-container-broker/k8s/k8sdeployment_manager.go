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
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/apis/extensions"
	clientK8s "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/util/wait"

	"github.com/trustedanalytics/tap-ceph-broker/client"
	"github.com/trustedanalytics/tap-container-broker/models"
)

type DeploymentManager interface {
	DeleteAll(selector labels.Selector) error
	UpdateReplicasNumber(name string, count int) error
	Create(replicationController *extensions.Deployment) (*extensions.Deployment, error)
	List(selector labels.Selector) (*extensions.DeploymentList, error)
}

type DeploymentConnector struct {
	client     ExtensionsInterface
	cephClient client.CephBroker
}

func NewDeploymentControllerManager(client ExtensionsInterface, cephClient client.CephBroker) *DeploymentConnector {
	return &DeploymentConnector{client: client, cephClient: cephClient}
}

func (r *DeploymentConnector) DeleteAll(selector labels.Selector) error {
	logger.Debug("Delete deployment selector:", selector)
	deployments, err := r.List(selector)
	if err != nil {
		logger.Error("List deployment failed:", err)
		return err
	}

	for _, deployment := range deployments.Items {
		name := deployment.ObjectMeta.Name

		if err := r.UpdateReplicasNumber(name, 0); err != nil {
			logger.Error("UpdateReplicasNumber for deployment failed:", err)
			return err
		}

		if err := processDeploymentVolumes(deployment, r.cephClient, false); err != nil {
			return err
		}

		logger.Debug("Deleting deployment:", name)
		err = r.client.Deployments(api.NamespaceDefault).Delete(name, &api.DeleteOptions{})
		if err != nil {
			logger.Error("Delete deployment failed:", err)
			return err
		}
	}
	return nil
}

func (r *DeploymentConnector) UpdateReplicasNumber(name string, count int) error {
	logger.Infof("Set replicas to %d. Deployment name: %s", count, name)
	deployment, err := r.client.Deployments(api.NamespaceDefault).Get(name)
	if err != nil {
		return err
	}

	deployment.Spec.Replicas = int32(count)
	if _, err = r.client.Deployments(api.NamespaceDefault).Update(deployment); err != nil {
		return err
	}

	return wait.PollImmediate(models.ProcessorsIntervalSec, models.ProcessorsTotalDuration, clientK8s.DeploymentHasDesiredReplicas(r.client, deployment))
}

func (r *DeploymentConnector) Create(deployment *extensions.Deployment) (*extensions.Deployment, error) {
	return r.client.Deployments(api.NamespaceDefault).Create(deployment)
}

func (r *DeploymentConnector) List(selector labels.Selector) (*extensions.DeploymentList, error) {
	logger.Debug("List DeploymentList selector:", selector)
	return r.client.Deployments(api.NamespaceDefault).List(api.ListOptions{
		LabelSelector: selector,
	})
}
