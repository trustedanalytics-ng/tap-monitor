package k8s

import (
	"os"
	"strconv"
	"strings"
)

func ConvertToProperEnvName(key string) string {
	return strings.Replace(key, "-", "_", -1)
}

func getCephImageSize() uint64 {
	cephEnvValue := os.Getenv("CEPH_IMAGE_SIZE_MB")
	if cephEnvValue != "" {
		size, err := strconv.ParseUint(cephEnvValue, 0, 64)
		if err != nil {
			logger.Errorf("Can't parse value: %s of CEPH_IMAGE_SIZE_MB env - default limit: %d will be used", cephEnvValue, defaultCephImageSizeMB)
			return defaultCephImageSizeMB
		}
		return size
	}
	return defaultCephImageSizeMB
}
