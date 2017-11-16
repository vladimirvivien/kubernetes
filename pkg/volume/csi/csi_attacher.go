/*
Copyright 2017 The Kubernetes Authors.

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
	"errors"
	"time"

	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/kubernetes/pkg/volume"
)

type csiAttacher struct {
}

// volume.Attacher methods
var _ volume.Attacher = &csiAttacher{}

func (c *csiAttacher) Attach(spec *volume.Spec, nodeName types.NodeName) (string, error) {
	return "", errors.New("unimplemented")
}

func (c *csiAttacher) VolumesAreAttached(specs []*volume.Spec, nodeName types.NodeName) (map[*volume.Spec]bool, error) {
	return nil, errors.New("unimplemented")
}

func (c *csiAttacher) WaitForAttach(spec *volume.Spec, devicePath string, pod *v1.Pod, timeout time.Duration) (string, error) {
	return "", errors.New("unimplemented")
}

func (c *csiAttacher) GetDeviceMountPath(spec *volume.Spec) (string, error) {
	return "", errors.New("unimplemented")
}

func (c *csiAttacher) MountDevice(spec *volume.Spec, devicePath string, deviceMountPath string) error {
	return errors.New("unimplemented")
}

var _ volume.Detacher = &csiAttacher{}

func (c *csiAttacher) Detach(deviceName string, nodeName types.NodeName) error {
	return errors.New("unimplemented")
}

func (c *csiAttacher) UnmountDevice(deviceMountPath string) error {
	return errors.New("unimplemented")
}
