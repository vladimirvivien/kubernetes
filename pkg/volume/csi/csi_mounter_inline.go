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
	"errors"
	"fmt"
	"strings"
	"time"

	"k8s.io/klog"

	api "k8s.io/api/core/v1"
	storage "k8s.io/api/storage/v1beta1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
)

// setUpInline handles the provisioning and attachment of inline volumes embedded
// in a pod spec.  The method returns attachmentID, volumeHandle that was used
// during the inline setup.
//
// It follows these steps:
// 1. Check for volumeHandle
// - If not provided assume auto-provision, continue to provisioning #2
// - If provided, continue to attachment #3
// 2. Provisioning
// - Use volSpecName to initiate request to create vol with csi.CreateVolume()
// - return response.ID as volHandle
// 3. Attachment
// - Call csiPlugin.csiAttacher.Attach()
// - Use volHandle above to generate attachID
// - Wait for VolumeAttachment.ID from csiAttacher.Attach()
// - if attachment ok, return attachID.
func (c *csiMountMgr) setUpInline(csiSource *api.CSIVolumeSource) (string, string, error) {
	klog.V(4).Info(log("mounter.setupInline called for CSIVolumeSource"))

	if csiSource == nil {
		return "", "", errors.New("missing CSIVolumeSource")
	}

	var (
		driverName = c.driverName
		volHandle  = csiSource.VolumeHandle
	)

	// missing volHandle means we should provision
	if volHandle == nil {
		klog.V(4).Info(log("mounter.setupInline failed as CSIVolumeSource.VolumeHandle provided"))
		return "", "", fmt.Errorf("mounter.setupInline failed as CSIVolumeSource.VolumeHandle provided")
	}

	skip, err := c.plugin.skipAttach(driverName)
	if err != nil {
		klog.Error(log("mounter.setupInline failed to get attachability setting for driver: %v", err))
		return "", "", err
	}

	// trigger attachment and wait for attach.Name (if necessary)
	if skip {
		klog.V(4).Info(log("mounter.setupInline skipping volume attachment"))
		return "", *volHandle, nil
	}

	attachID, err := c.inlineAttach(*volHandle, csiSource, csiDefaultTimeout)
	if err != nil {
		return "", "", err
	}

	return attachID, *volHandle, nil
}

// inlineAttach will create and post a VolumeAttachment API object
// to signal attachment to the external-attacher.  This subsequently causes
// the external-attacher to contact the CSI driver to create the attachment.
// Returns the ID for the VolumeAttachment API ovbject or an error if failure.
func (c *csiMountMgr) inlineAttach(volHandle string, csiSource *api.CSIVolumeSource, attachTimeout time.Duration) (string, error) {
	klog.V(4).Info(log("mounter.inlineAttach called for volumeHandle %s", volHandle))

	if csiSource == nil {
		return "", errors.New("missing inline CSIVolumeSource")
	}
	nodeName := string(c.plugin.host.GetNodeName())
	driverName := csiSource.Driver
	attachID := getAttachmentName(volHandle, driverName, nodeName)
	namespace := c.pod.Namespace
	volSource := c.spec.Volume

	attacher := &csiAttacher{
		k8s:           c.k8s,
		plugin:        c.plugin,
		waitSleepTime: 5 * time.Second,
	}

	attachment := &storage.VolumeAttachment{
		ObjectMeta: meta.ObjectMeta{
			Name: attachID,
		},
		Spec: storage.VolumeAttachmentSpec{
			NodeName: nodeName,
			Attacher: driverName,
			Source: storage.VolumeAttachmentSource{
				InlineVolumeSource: &storage.InlineVolumeSource{
					VolumeSource: volSource.VolumeSource,
					Namespace:    namespace,
				},
			},
		},
		Status: storage.VolumeAttachmentStatus{Attached: false},
	}

	// create and wait for attachment
	attachID, err := attacher.postVolumeAttachment(driverName, volHandle, attachment, attachTimeout)
	if err != nil {
		klog.Errorf(log("mount.inlineAttach failed to post attachment [attachID %s]: %v", attachID, err))
		return "", err
	}

	klog.V(4).Info(log("mounter.inlineAttach attached OK [driver:%s, volumeHandle: %s, attachID:%s]", driverName, volHandle, attachID))
	return attachID, nil
}

func generateVolHandle(prefix string, size int) string {
	return fmt.Sprintf("%s-%s", prefix, strings.Replace(string(uuid.NewUUID()), "-", "", -1)[0:size])
}
