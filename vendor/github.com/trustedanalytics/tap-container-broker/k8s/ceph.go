package k8s

import (
	"errors"
	"fmt"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/apis/extensions"

	"github.com/trustedanalytics/tap-ceph-broker/client"
	cephModel "github.com/trustedanalytics/tap-ceph-broker/model"
)

func processDeploymentVolumes(deployment extensions.Deployment, cephClient client.CephBroker, isCreateAction bool) error {
	for _, volume := range deployment.Spec.Template.Spec.Volumes {
		if volume.RBD != nil {
			if isCreateAction {
				if err := addVolumeToCeph(volume, cephClient); err != nil {
					return err
				}
			} else {
				if err := removeVolumeFromCeph(volume, cephClient); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func addVolumeToCeph(volume api.Volume, cephClient client.CephBroker) error {
	logger.Debug("CEPH volume creation: ", volume.RBD.RBDImage)
	status, err := cephClient.CreateRBD(cephModel.RBD{
		ImageName:  volume.RBD.RBDImage,
		Size:       getCephImageSize(),
		FileSystem: volume.RBD.FSType,
	})
	if err != nil || status != 200 {
		message := fmt.Sprintf("CEPH volume creation error, image name: %s, statusCode: %d, error: %v", volume.RBD.RBDImage, status, err)
		logger.Errorf(message)
		return errors.New(message)
	}
	return nil
}

func removeVolumeFromCeph(volume api.Volume, cephClient client.CephBroker) error {
	logger.Debug("CEPH volume deletion: ", volume.RBD.RBDImage)
	status, err := cephClient.DeleteRBD(volume.RBD.RBDImage)
	if err != nil || status != 204 {
		message := fmt.Sprintf("CEPH volume deletion error, image name: %s, statusCode: %d, error: %v", volume.RBD.RBDImage, status, err)
		logger.Errorf(message)
		return errors.New(message)
	}
	return nil
}
