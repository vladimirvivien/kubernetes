/*
Copyright 2018 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package csi

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"time"

	api "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog"
	"k8s.io/kubernetes/pkg/volume"
	utilstrings "k8s.io/utils/strings"
)

const (
	testInformerSyncPeriod  = 100 * time.Millisecond
	testInformerSyncTimeout = 30 * time.Second
)

// backoffOptions returns a pre-configured "k8s.io/apimachinery/pkg/util/wait".Backoff
// with the following values:
//
// Duration = time.Second * 10
// Factor   = 1.2
// Steps    = 10
func defaultBackoff() wait.Backoff {
	return wait.Backoff{
		Duration: time.Second * 10,
		Factor:   1.2,
		Steps:    10,
	}
}

func getCredentialsFromSecret(k8s kubernetes.Interface, secretRef *api.SecretReference) (map[string]string, error) {
	credentials := map[string]string{}
	secret, err := k8s.CoreV1().Secrets(secretRef.Namespace).Get(secretRef.Name, meta.GetOptions{})
	if err != nil {
		klog.Errorf("failed to find the secret %s in the namespace %s with error: %v\n", secretRef.Name, secretRef.Namespace, err)
		return credentials, err
	}
	for key, value := range secret.Data {
		credentials[key] = string(value)
	}
	return credentials, nil
}

// saveVolumeData persists parameter data as json file at the provided location
func saveVolumeData(dir string, fileName string, data map[string]string) error {
	dataFilePath := path.Join(dir, fileName)
	klog.V(4).Info(log("saving volume data file [%s]", dataFilePath))
	file, err := os.Create(dataFilePath)
	if err != nil {
		klog.Error(log("failed to save volume data file %s: %v", dataFilePath, err))
		return err
	}
	defer file.Close()
	if err := json.NewEncoder(file).Encode(data); err != nil {
		klog.Error(log("failed to save volume data file %s: %v", dataFilePath, err))
		return err
	}
	klog.V(4).Info(log("volume data file saved successfully [%s]", dataFilePath))
	return nil
}

// loadVolumeData loads volume info from specified json file/location
func loadVolumeData(dir string, fileName string) (map[string]string, error) {
	// remove /mount at the end
	dataFileName := path.Join(dir, fileName)
	klog.V(4).Info(log("loading volume data file [%s]", dataFileName))

	file, err := os.Open(dataFileName)
	if err != nil {
		klog.Error(log("failed to open volume data file [%s]: %v", dataFileName, err))
		return nil, err
	}
	defer file.Close()
	data := map[string]string{}
	if err := json.NewDecoder(file).Decode(&data); err != nil {
		klog.Error(log("failed to parse volume data file [%s]: %v", dataFileName, err))
		return nil, err
	}

	return data, nil
}

func getSourceFromSpec(spec *volume.Spec) (*api.CSIVolumeSource, *api.CSIPersistentVolumeSource, error) {
	if spec == nil {
		return nil, nil, fmt.Errorf("volume.Spec missing volume spec")
	}

	if spec.Volume != nil &&
		spec.Volume.CSI != nil {
		return spec.Volume.CSI, nil, nil
	}
	if spec.PersistentVolume != nil &&
		spec.PersistentVolume.Spec.CSI != nil {
		return nil, spec.PersistentVolume.Spec.CSI, nil
	}

	return nil, nil, fmt.Errorf("volume.Spec missing volume source and persistent volume source")
}

func getCSISourceFromSpec(spec *volume.Spec) (*api.CSIPersistentVolumeSource, error) {
	if spec.PersistentVolume != nil &&
		spec.PersistentVolume.Spec.CSI != nil {
		return spec.PersistentVolume.Spec.CSI, nil
	}

	return nil, fmt.Errorf("CSIPersistentVolumeSource not defined in spec")
}

func getReadOnlyFromSpec(spec *volume.Spec) (bool, error) {
	if spec.PersistentVolume != nil &&
		spec.PersistentVolume.Spec.CSI != nil {
		return spec.ReadOnly, nil
	}

	return false, fmt.Errorf("CSIPersistentVolumeSource not defined in spec")
}

// log prepends log string with `kubernetes.io/csi`
func log(msg string, parts ...interface{}) string {
	return fmt.Sprintf(fmt.Sprintf("%s: %s", csiPluginName, msg), parts...)
}

// getVolumeDevicePluginDir returns the path where the CSI plugin keeps the
// symlink for a block device associated with a given specVolumeID.
// path: plugins/kubernetes.io/csi/volumeDevices/{specVolumeID}/dev
func getVolumeDevicePluginDir(specVolID string, host volume.VolumeHost) string {
	sanitizedSpecVolID := utilstrings.EscapeQualifiedName(specVolID)
	return path.Join(host.GetVolumeDevicePluginDir(csiPluginName), sanitizedSpecVolID, "dev")
}

// getVolumeDeviceDataDir returns the path where the CSI plugin keeps the
// volume data for a block device associated with a given specVolumeID.
// path: plugins/kubernetes.io/csi/volumeDevices/{specVolumeID}/data
func getVolumeDeviceDataDir(specVolID string, host volume.VolumeHost) string {
	sanitizedSpecVolID := utilstrings.EscapeQualifiedName(specVolID)
	return path.Join(host.GetVolumeDevicePluginDir(csiPluginName), sanitizedSpecVolID, "data")
}

// hasReadWriteOnce returns true if modes contains v1.ReadWriteOnce
func hasReadWriteOnce(modes []api.PersistentVolumeAccessMode) bool {
	if modes == nil {
		return false
	}
	for _, mode := range modes {
		if mode == api.ReadWriteOnce {
			return true
		}
	}
	return false
}
