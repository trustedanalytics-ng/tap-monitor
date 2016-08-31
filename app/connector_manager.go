package app

import (
	"os"
	"strings"

	catalogApi "github.com/trustedanalytics/tapng-catalog/client"
	"github.com/trustedanalytics/tapng-container-broker/k8s"
	"errors"
	"fmt"
)

type ConnectionConfig struct {
	KubernetesApi         k8s.KubernetesApi
	CatalogApi            catalogApi.TapCatalogApi
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
	address := getAddressFromK8SEnvs("CATALOG")
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

func GetQueueConnectionString() string {
	address := getAddressFromK8SEnvs("QUEUE")
	user := os.Getenv("QUEUE_USER")
	pass := os.Getenv("QUEUE_PASS")
	return fmt.Sprintf("amqp://%v:%v@%v/", user, pass, address)
}

func getAddressFromK8SEnvs(componentName string) string {
	serviceName := os.Getenv(componentName + "_KUBERNETES_SERVICE_NAME")
	if serviceName != "" {
		hostname := os.Getenv(strings.ToUpper(serviceName) + "_SERVICE_HOST")
		if hostname != "" {
			port := os.Getenv(strings.ToUpper(serviceName) + "_SERVICE_PORT")
			return hostname + ":" + port
		}
	}
	return "localhost" + ":" + os.Getenv(componentName + "_PORT")
}
