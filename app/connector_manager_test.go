package app

import (
	"errors"
	"net/http"
	"testing"

	"github.com/golang/mock/gomock"
	. "github.com/smartystreets/goconvey/convey"

	catalogApi "github.com/trustedanalytics/tap-catalog/client"
	"github.com/trustedanalytics/tap-container-broker/k8s"
)

const (
	CatalogUser            = "sample_user"
	CatalogPassword        = "sample_password"
	SSLCertFileLocation    = "/root/cert"
	SSLCertKeyFileLocation = "/root/key"
	SSLCertCAFileLocation  = "/root/ca"
	AddressFromKubernetes  = "someaddress.com"
	K8SAPIAddress          = "k8s.someaddress.com"
	K8SAPIUser             = "k8s username"
	K8SAPIPassword         = "k8s password"
)

var basicEnvs = map[string]string{"CATALOG_USER": CatalogUser, "CATALOG_PASS": CatalogPassword, "K*S_API_ADDRESS": K8SAPIAddress, "K8S_API_USERNAME": K8SAPIUser, "K8S_API_PASSWORD": K8SAPIPassword}
var certEnvs = map[string]string{"CATALOG_SSL_CERT_FILE_LOCATION": SSLCertFileLocation, "CATALOG_SSL_KEY_FILE_LOCATION": SSLCertKeyFileLocation, "CATALOG_SSL_CA_FILE_LOCATION": SSLCertCAFileLocation}
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

func fakeNewTapCatalogAPIWithSSLAndBasicAuth(address, username, password, certPemFile, keyPemFile, caPemFile string) (*catalogApi.TapCatalogApiConnector, error) {
	return &catalogApi.TapCatalogApiConnector{Address: address, Username: username, Password: password, Client: &http.Client{}}, nil
}

func fakeNewTapCatalogAPIWithBasicAuth(address, username, password string) (*catalogApi.TapCatalogApiConnector, error) {
	return &catalogApi.TapCatalogApiConnector{Address: address, Username: username, Password: password, Client: &http.Client{}}, nil
}

func fakeGetNewK8FabricatorInstance(creds k8s.K8sClusterCredentials) (*k8s.K8Fabricator, error) {
	return &k8s.K8Fabricator{}, nil
}

func fakeGetNewK8FabricatorInstanceFailing(creds k8s.K8sClusterCredentials) (*k8s.K8Fabricator, error) {
	return nil, errors.New("error!")
}

func prepareTestingEnvironment(t *testing.T, useSSL bool) {
	getEnv = fakeGetEnv
	getAddressFromKubernetesEnvs = fakeGetAddressFromKubernetesEnvs

	localEnvs = make(map[string]string)
	for k, v := range basicEnvs {
		localEnvs[k] = v
	}
	if useSSL {
		for k, v := range certEnvs {
			localEnvs[k] = v
		}
	}

	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	newTapCatalogApiWithSSLAndBasicAuth = fakeNewTapCatalogAPIWithSSLAndBasicAuth
	newTapCatalogApiWithBasicAuth = fakeNewTapCatalogAPIWithBasicAuth
	getNewK8FabricatorInstance = fakeGetNewK8FabricatorInstance
}

func TestGetCatalogConnector(t *testing.T) {
	Convey("When there is no SSL ceritifiate", t, func() {
		prepareTestingEnvironment(t, false)

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

	Convey("When there is SSL ceritifiate", t, func() {
		prepareTestingEnvironment(t, true)

		Convey("getCatalogConnector should return proper response", func() {
			connector, err := getCatalogConnector()

			Convey("Error should be nil", func() {
				So(err, ShouldBeNil)
			})
			Convey("Address should be proper", func() {
				So(connector.Address, ShouldEqual, "https://"+AddressFromKubernetes)
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

func TestInitConnection(t *testing.T) {
	prepareTestingEnvironment(t, false)
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
			getNewK8FabricatorInstance = fakeGetNewK8FabricatorInstanceFailing

			Convey("InitConnections should return error", func() {
				err := InitConnections()

				Convey("err should be nil", func() {
					So(err, ShouldNotBeNil)
				})
			})
		})
	})
}
