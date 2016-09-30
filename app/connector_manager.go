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
	"os"

	catalogApi "github.com/trustedanalytics/tap-catalog/client"
	"github.com/trustedanalytics/tap-container-broker/k8s"
	"github.com/trustedanalytics/tap-go-common/util"
)

type ConnectionConfig struct {
	KubernetesApi k8s.KubernetesApi
	CatalogApi    catalogApi.TapCatalogApi
}

var config *ConnectionConfig

var getAddressFromKubernetesEnvs = util.GetAddressFromKubernetesEnvs
var getEnv = os.Getenv
var newTapCatalogApiWithSSLAndBasicAuth = catalogApi.NewTapCatalogApiWithSSLAndBasicAuth
var newTapCatalogApiWithBasicAuth = catalogApi.NewTapCatalogApiWithBasicAuth
var getNewK8FabricatorInstance = k8s.GetNewK8FabricatorInstance

func InitConnections() error {
	catalogConnector, err := getCatalogConnector()
	if err != nil {
		return errors.New("Can't connect with TAP-NG-catalog!" + err.Error())
	}

	kubernetesApiConnector, err := getNewK8FabricatorInstance(k8s.K8sClusterCredentials{
		Server:   getEnv("K8S_API_ADDRESS"),
		Username: getEnv("K8S_API_USERNAME"),
		Password: getEnv("K8S_API_PASSWORD"),
	})

	if err != nil {
		return errors.New("Can't connect with K8S!" + err.Error())
	}

	config = &ConnectionConfig{}
	config.CatalogApi = catalogConnector
	config.KubernetesApi = kubernetesApiConnector

	return nil
}

func getCatalogConnector() (*catalogApi.TapCatalogApiConnector, error) {
	address := getAddressFromKubernetesEnvs("CATALOG")
	if getEnv("CATALOG_SSL_CERT_FILE_LOCATION") != "" {
		return newTapCatalogApiWithSSLAndBasicAuth(
			"https://"+address,
			getEnv("CATALOG_USER"),
			getEnv("CATALOG_PASS"),
			getEnv("CATALOG_SSL_CERT_FILE_LOCATION"),
			getEnv("CATALOG_SSL_KEY_FILE_LOCATION"),
			getEnv("CATALOG_SSL_CA_FILE_LOCATION"),
		)
	} else {
		return newTapCatalogApiWithBasicAuth(
			"http://"+address,
			getEnv("CATALOG_USER"),
			getEnv("CATALOG_PASS"),
		)
	}
}
