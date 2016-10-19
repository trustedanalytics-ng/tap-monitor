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
	"net/http"
	"testing"

	"github.com/golang/mock/gomock"
	. "github.com/smartystreets/goconvey/convey"

	catalogApi "github.com/trustedanalytics/tap-catalog/client"
	cephApi "github.com/trustedanalytics/tap-ceph-broker/client"
	kubernetesApi "github.com/trustedanalytics/tap-container-broker/k8s"
	templateRepositoryApi "github.com/trustedanalytics/tap-template-repository/client"
)

const (
	CatalogUser                = "sample_user"
	CatalogPassword            = "sample_password"
	TemplateRepositoryUser     = "sample_user"
	TemplateRepositoryPassword = "sample_password"
	AddressFromKubernetes      = "someaddress.com"
	K8SAPIAddress              = "k8s.someaddress.com"
	K8SAPIUser                 = "k8s username"
	K8SAPIPassword             = "k8s password"
)

var basicEnvs = map[string]string{
	"CATALOG_USER":             CatalogUser,
	"CATALOG_PASS":             CatalogPassword,
	"TEMPLATE_REPOSITORY_USER": TemplateRepositoryUser,
	"TEMPLATE_REPOSITORY_PASS": TemplateRepositoryPassword,
	"K8S_API_ADDRESS":          K8SAPIAddress,
	"K8S_API_USERNAME":         K8SAPIUser,
	"K8S_API_PASSWORD":         K8SAPIPassword,
}

var localEnvs map[string]string

func fakeGetEnv(key string) string {
	if val, ok := localEnvs[key]; ok {
		return val
	}
	return ""
}

func fakeGetAddressFromKubernetesEnvs(key string) string {
	return AddressFromKubernetes
}

func fakeNewTapCatalogAPIWithBasicAuth(address, username, password string) (*catalogApi.TapCatalogApiConnector, error) {
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
	getAddressFromKubernetesEnvs = fakeGetAddressFromKubernetesEnvs

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
	Convey("When there is no SSL ceritifiate", t, func() {
		prepareTestingEnvironment(t)

		Convey("getCatalogConnector should return proper response", func() {
			connector, err := getCatalogConnector()

			Convey("Error should be nil", func() {
				So(err, ShouldBeNil)
			})
			Convey("Address should be proper", func() {
				So(connector.Address, ShouldEqual, "http://"+AddressFromKubernetes)
			})
			Convey("Catalog should be proper", func() {
				So(connector.Username, ShouldEqual, CatalogUser)
			})
			Convey("Password should be proper", func() {
				So(connector.Password, ShouldEqual, CatalogPassword)
			})
			Convey("Client should not be nil", func() {
				So(connector.Client, ShouldNotBeNil)
			})
		})
	})
}

func TestGetTemplateRepositoryConnector(t *testing.T) {
	Convey("When there is no SSL ceritifiate", t, func() {
		prepareTestingEnvironment(t)

		Convey("getTemplateRepositoryConnector should return proper response", func() {
			connector, err := getTemplateRepositoryConnector()

			Convey("Error should be nil", func() {
				So(err, ShouldBeNil)
			})
			Convey("Address should be proper", func() {
				So(connector.Address, ShouldEqual, "http://"+AddressFromKubernetes)
			})
			Convey("User should be proper", func() {
				So(connector.Username, ShouldEqual, TemplateRepositoryUser)
			})
			Convey("Password should be proper", func() {
				So(connector.Password, ShouldEqual, TemplateRepositoryPassword)
			})
			Convey("Client should not be nil", func() {
				So(connector.Client, ShouldNotBeNil)
			})
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
