# TAP Monitor
Monitor is a microservice developed to be a part of TAP platform.
It monitors the changes in TAP-catalog (REST) and provide them to TAP-Container-broker and TAP-Image-factory by sending messeges on the specific queues (rabbitMQ).
In the future above changes will be verify in Kubernetes by using K8sFabricator lib from Container Broker.

## REQUIREMENTS

### Dependencies
This component depends on and communicates with:
* [tap-catalog](https://github.com/intel-data/tap-catalog)
* [tap-template-repository](https://github.com/intel-data/tap-template-repository)
* [tap-container-broker](https://github.com/intel-data/tap-container-broker)
* [tap-image-factory](https://github.com/intel-data/tap-image-factory)
* [tap-ceph-broker](https://github.com/intel-data/tap-ceph-broker)
* RabbitMQ

### Compilation
* git (for pulling repository)
* go >= 1.6

## Compilation
To build project:
```
  git clone https://github.com/intel-data/tap-monitor
  cd tap-monitor
  make build_anywhere
```
Binaries are available in ./application directory.

## USAGE
As Container Broker depends on external components, following environment variables should be set:
* CATALOG_KUBERNETES_SERVICE_NAME
* CATALOG_USER
* CATALOG_PASS
* TEMPLATE_REPOSITORY_KUBERNETES_SERVICE_NAME
* TEMPLATE_REPOSITORY_USER
* TEMPLATE_REPOSITORY_PASS
* CEPH_BROKER_KUBERNETES_SERVICE_NAME
* CEPH_BROKER_USER
* CEPH_BROKER_PASS
* K8S_API_ADDRESS
* K8S_API_USERNAME
* K8S_API_PASSWORD
* QUEUE_KUBERNETES_SERVICE_NAME
* QUEUE_USER
* QUEUE_PASS

Other required environment variables:
* IMAGE_FACTORY_HUB_ADDRESS - defines address where Image Factory will place Image
* GENERIC_SERVICE_TEMPLATE_ID - defines Id of default template for offering creation in Template Repository

## FLOW
Monitor works automatically and doesn't serve any REST/RPC endpoints. 
Monitor observes Catalog Instances and Images states and trigger specific acction depends of state value:
* Instance state REQUESTED - message with request key "create" will be sent to Container Broker queue
* Instance state DESTROY_REQ - message with request key "delete" will be sent to Container Broker queue
* Image state PENDING - message with request key "image" will be sent to Image Repository queue
* Image state READY - depends of Image type:
    * app - new Instance with state REQUESTED will be created in Catalog
    * service - new Template will be added to Catalog and Template Repository, Offering state will be updated to READY/OFFLINE
