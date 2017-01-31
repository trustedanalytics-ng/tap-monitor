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

import (
	"testing"

	"k8s.io/kubernetes/pkg/apis/extensions"
	"k8s.io/kubernetes/pkg/api"
	. "github.com/smartystreets/goconvey/convey"

	templateModels "github.com/trustedanalytics/tap-template-repository/model"
	kubernetesApi "github.com/trustedanalytics/tap-container-broker/k8s"
)

const (
	deploymentName  = "test-deployment"
)


func TestK8SObjectMethods(t *testing.T) {
	Convey("Test getId", t, func() {
		entity := getK8SObject()

		Convey("Should return empty respone when label is not set", func() {
			id := entity.getId()
			So(id, ShouldBeEmpty)
		})

		Convey("Should return label value as Id when label is not empty", func() {
			labels := make(map[string]string)
			labels[kubernetesApi.InstanceIdLabel] = "sample-id"
			entity.Deployments[0].Labels = labels

			id := entity.getId()
			So(id, ShouldEqual, "sample-id")
		})
	})

	Convey("Test isFullyRunning", t, func() {
		entity := getK8SObject()

		Convey("Should return true if all deployments replica values are greater then 0", func() {
			isRunning := entity.isFullyRunning()
			So(isRunning, ShouldBeTrue)
		})

		Convey("Should return false if any of deployments replica vakue is equal 0", func() {
			entity.Deployments[0].Spec.Replicas = 0
			isRunning := entity.isFullyRunning()
			So(isRunning, ShouldBeFalse)
		})
	})

	Convey("Test isFullyStopped", t, func() {
		entity := getK8SObject()

		Convey("Should return true if all deployments replica values are equal 0", func() {
			entity.Deployments[0].Spec.Replicas = 0
			isStopped := entity.isFullyStopped()
			So(isStopped, ShouldBeTrue)
		})

		Convey("Should return false if any of deployments replica value is greater then 0", func() {
			isStopped := entity.isFullyStopped()
			So(isStopped, ShouldBeFalse)
		})
	})
}

func getK8SObject() *K8SObject {
	deployment := extensions.Deployment{
		ObjectMeta: api.ObjectMeta{
			Name: deploymentName,
		},
		Spec: extensions.DeploymentSpec{
			Replicas: 1,
		},
	}

	return &K8SObject{
		Deployments: []*extensions.Deployment{&deployment},
		Type:        templateModels.ComponentTypeInstance,
	}
}