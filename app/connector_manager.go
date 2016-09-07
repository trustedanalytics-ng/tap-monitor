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

func InitConnections() error {
	catalogConnector, err := getCatalogConnector()
	if err != nil {
		return errors.New("Can't connect with TAP-NG-catalog!" + err.Error())
	}

	kubernetesApiConnector, err := k8s.GetNewK8FabricatorInstance(k8s.K8sClusterCredentials{
		Server:   os.Getenv("K8S_API_ADDRESS"),
		Username: os.Getenv("K8S_API_USERNAME"),
		Password: os.Getenv("K8S_API_PASSWORD"),
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
	address := util.GetAddressFromKubernetesEnvs("CATALOG")
	if os.Getenv("CATALOG_SSL_CERT_FILE_LOCATION") != "" {
		return catalogApi.NewTapCatalogApiWithSSLAndBasicAuth(
			"https://"+address,
			os.Getenv("CATALOG_USER"),
			os.Getenv("CATALOG_PASS"),
			os.Getenv("CATALOG_SSL_CERT_FILE_LOCATION"),
			os.Getenv("CATALOG_SSL_KEY_FILE_LOCATION"),
			os.Getenv("CATALOG_SSL_CA_FILE_LOCATION"),
		)
	} else {
		return catalogApi.NewTapCatalogApiWithBasicAuth(
			"http://"+address,
			os.Getenv("CATALOG_USER"),
			os.Getenv("CATALOG_PASS"),
		)
	}
}
