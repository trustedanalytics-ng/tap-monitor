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
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/golang/mock/gomock"
	. "github.com/smartystreets/goconvey/convey"

	catalogApi "github.com/trustedanalytics-ng/tap-catalog/client"
	cephApi "github.com/trustedanalytics-ng/tap-ceph-broker/client"
	kubernetesApi "github.com/trustedanalytics-ng/tap-container-broker/k8s"
	templateRepositoryApi "github.com/trustedanalytics-ng/tap-template-repository/client"
)

const (
	catalogUser                = "sample_user"
	catalogPassword            = "sample_password"
	catalogHost                = "catalog.internal"
	catalogPort                = "443"
	templateRepositoryHost     = "template-repository.internal"
	templateRepositoryPort     = "443"
	templateRepositoryUser     = "sample_user"
	templateRepositoryPassword = "sample_password"
	k8sAPIAddress              = "k8s.someaddress.com"
	k8sAPIUser                 = "k8s-username"
	k8sAPIPassword             = "k8s-password"
	cephBrokerHost             = "ceph-broker.internal"
	cephBrokerPort             = "80"
	cephBrokerUser             = "sample_ceph_broker_user"
	cephBrokerPass             = "sample_ceph_broker_pass"
)

var basicEnvs = map[string]string{
	"CATALOG_HOST":             catalogHost,
	"CATALOG_PORT":             catalogPort,
	"CATALOG_USER":             catalogUser,
	"CATALOG_PASS":             catalogPassword,
	"TEMPLATE_REPOSITORY_HOST": templateRepositoryHost,
	"TEMPLATE_REPOSITORY_PORT": templateRepositoryPort,
	"TEMPLATE_REPOSITORY_USER": templateRepositoryUser,
	"TEMPLATE_REPOSITORY_PASS": templateRepositoryPassword,
	"K8S_API_ADDRESS":          k8sAPIAddress,
	"K8S_API_USERNAME":         k8sAPIUser,
	"K8S_API_PASSWORD":         k8sAPIPassword,
	"CEPH_BROKER_HOST":         cephBrokerHost,
	"CEPH_BROKER_PORT":         cephBrokerPort,
	"CEPH_BROKER_USER":         cephBrokerUser,
	"CEPH_BROKER_PASS":         cephBrokerPass,
}

var localEnvs map[string]string

func fakeGetEnv(key string) string {
	if val, ok := localEnvs[key]; ok {
		return val
	}
	return ""
}

func fakeGetConnectionParametersFromEnv(componentName string) (string, string, string, error) {
	host := getEnv(componentName + "_HOST")
	port := getEnv(componentName + "_PORT")
	user := getEnv(componentName + "_USER")
	pass := getEnv(componentName + "_PASS")

	if user == "" || pass == "" || host == "" || port == "" {
		return fmt.Sprintf("%s:%s", host, port), user, pass, errors.New("undefined vars")
	}

	return fmt.Sprintf("%s:%s", host, port), user, pass, nil
}

func fakeNewTapCatalogAPIWithBasicAuth(address, username, password string) (catalogApi.TapCatalogApi, error) {
	return &catalogApi.TapCatalogApiConnector{Address: address, Username: username, Password: password, Client: &http.Client{}}, nil
}

func fakeGetNewK8FabricatorInstance(creds kubernetesApi.K8sClusterCredentials, cephClient cephApi.CephBroker) (*kubernetesApi.K8Fabricator, error) {
	return &kubernetesApi.K8Fabricator{}, nil
}

func fakeGetNewK8FabricatorInstanceFailing(creds kubernetesApi.K8sClusterCredentials, cephClient cephApi.CephBroker) (*kubernetesApi.K8Fabricator, error) {
	return nil, errors.New("error!")
}

func fakeNewTapTemplateRepositoryAPIWithBasicAuth(address, username, password string) (*templateRepositoryApi.TemplateRepositoryConnector, error) {
	return &templateRepositoryApi.TemplateRepositoryConnector{Address: address, Username: username, Password: password, Client: &http.Client{}}, nil
}

func prepareTestingEnvironment(t *testing.T) {
	getEnv = fakeGetEnv
	getConnectionParametersFromEnv = fakeGetConnectionParametersFromEnv

	localEnvs = make(map[string]string)
	for k, v := range basicEnvs {
		localEnvs[k] = v
	}

	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	newTapCatalogApiWithBasicAuth = fakeNewTapCatalogAPIWithBasicAuth

	newTapTemplateRepositoryWithBasicAuth = fakeNewTapTemplateRepositoryAPIWithBasicAuth

	newK8FabricatorInstance = fakeGetNewK8FabricatorInstance
}

func TestGetCatalogConnector(t *testing.T) {
	prepareTestingEnvironment(t)

	Convey("getCatalogConnector should return proper response", t, func() {
		_, err := getCatalogConnector()

		Convey("Error should be nil", func() {
			So(err, ShouldBeNil)
		})
	})
}

func TestGetTemplateRepositoryConnector(t *testing.T) {
	prepareTestingEnvironment(t)

	Convey("getTemplateRepositoryConnector should return proper response", t, func() {
		connector, err := getTemplateRepositoryConnector()

		Convey("Error should be nil", func() {
			So(err, ShouldBeNil)
		})
		Convey("Address should be proper", func() {
			So(connector.Address, ShouldEqual, fmt.Sprintf("https://%s:%s", templateRepositoryHost,
				templateRepositoryPort))
		})
		Convey("User should be proper", func() {
			So(connector.Username, ShouldEqual, templateRepositoryUser)
		})
		Convey("Password should be proper", func() {
			So(connector.Password, ShouldEqual, templateRepositoryPassword)
		})
		Convey("Client should not be nil", func() {
			So(connector.Client, ShouldNotBeNil)
		})
	})
}

func TestInitConnection(t *testing.T) {
	prepareTestingEnvironment(t)
	testCatalogConnector, _ := getCatalogConnector()

	Convey("For initial config set to nil", t, func() {
		config = nil

		Convey("InitConnections call should set config properly", func() {
			err := InitConnections()

			Convey("err should not be nil", func() {
				So(err, ShouldBeNil)

				Convey("config should not be nil", func() {
					So(config, ShouldNotBeNil)

					Convey("CatalogApi should be set properly", func() {
						So(config.CatalogApi, ShouldResemble, testCatalogConnector)
					})

					Convey("KubernetesApi should not be nil", func() {
						So(config.KubernetesApi, ShouldNotBeNil)
					})
				})
			})
		})
		Convey("When getNewK8FabricatorInstance fails", func() {
			newK8FabricatorInstance = fakeGetNewK8FabricatorInstanceFailing

			Convey("InitConnections should return error", func() {
				err := InitConnections()

				Convey("err should be nil", func() {
					So(err, ShouldNotBeNil)
				})
			})
		})
	})
}
