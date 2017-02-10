package k8s

import (
	"os"
	"strconv"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/apis/extensions"
	"k8s.io/kubernetes/pkg/util/intstr"

	commonHttp "github.com/trustedanalytics-ng/tap-go-common/http"
)

func MakeIngressForServices(instanceId, hostname string, services []api.Service) extensions.Ingress {
	rules := []extensions.IngressRule{}

	for _, service := range services {
		for _, port := range service.Spec.Ports {
			rule := extensions.IngressRule{
				Host: hostname + "-" + strconv.FormatInt(int64(port.Port), 10) + "." + os.Getenv("DOMAIN"),
				IngressRuleValue: extensions.IngressRuleValue{
					HTTP: &extensions.HTTPIngressRuleValue{
						Paths: []extensions.HTTPIngressPath{
							{
								Backend: extensions.IngressBackend{
									ServiceName: service.Name,
									ServicePort: intstr.FromInt(int(port.Port)),
								},
								Path: "/",
							},
						},
					},
				},
			}
			rules = append(rules, rule)
		}
	}

	return extensions.Ingress{
		ObjectMeta: getK8sObjectMeta(instanceId),
		Spec: extensions.IngressSpec{
			Rules: rules,
		},
	}
}

func MakeServiceForPorts(instanceId string, ports []int32) api.Service {
	servicePorts := []api.ServicePort{}
	for _, port := range ports {
		servicePort := api.ServicePort{
			Name:     strconv.FormatInt(int64(port), 10),
			Port:     port,
			Protocol: api.ProtocolTCP,
		}
		servicePorts = append(servicePorts, servicePort)
	}

	return api.Service{
		ObjectMeta: getK8sObjectMetaWithShortIdAsName(instanceId),
		Spec: api.ServiceSpec{
			Ports: servicePorts,
		},
	}
}

func MakeEndpointForPorts(instanceId, ip string, ports []int32) api.Endpoints {
	endpointPorts := []api.EndpointPort{}
	for _, port := range ports {
		endpointPort := api.EndpointPort{
			Name:     strconv.FormatInt(int64(port), 10),
			Port:     port,
			Protocol: api.ProtocolTCP,
		}
		endpointPorts = append(endpointPorts, endpointPort)
	}

	return api.Endpoints{
		ObjectMeta: getK8sObjectMetaWithShortIdAsName(instanceId),
		Subsets: []api.EndpointSubset{
			{
				Addresses: []api.EndpointAddress{
					{
						IP: ip,
					},
				},
				Ports: endpointPorts,
			},
		},
	}
}

func GetK8sTapLabels(instanceId string) map[string]string {
	labels := make(map[string]string)
	labels[managedByLabel] = managedByValue
	labels[InstanceIdLabel] = instanceId
	return labels
}

func getK8sObjectMeta(instanceId string) api.ObjectMeta {
	return api.ObjectMeta{
		Name:   instanceId,
		Labels: GetK8sTapLabels(instanceId),
	}
}

func getK8sObjectMetaWithShortIdAsName(instanceId string) api.ObjectMeta {
	return api.ObjectMeta{
		Name:   commonHttp.UuidToShortDnsName(instanceId),
		Labels: GetK8sTapLabels(instanceId),
	}
}
