/*
Copyright 2019 The Kubernetes Authors.

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

package storage

import (
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/onsi/ginkgo"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/test/e2e/framework"
	e2elog "k8s.io/kubernetes/test/e2e/framework/log"
	"k8s.io/kubernetes/test/e2e/storage/drivers"
	"k8s.io/kubernetes/test/e2e/storage/testpatterns"
	"k8s.io/kubernetes/test/e2e/storage/testsuites"
	"k8s.io/kubernetes/test/e2e/storage/utils"
)

var _ = utils.SIGDescribe("csi-inline-ephemeral", func() {
	f := framework.NewDefaultFramework("csi-inline-ephemeral")
	driver := drivers.InitHostPathCSIDriverForInline()

	var config *testsuites.PerTestConfig
	var testCleanup func()

	ginkgo.BeforeEach(func() {
		driver.SkipUnsupportedTest(testpatterns.TestPattern{})
		config, testCleanup = driver.PrepareTest(f)
	})

	ginkgo.AfterEach(func() {
		if testCleanup != nil {
			testCleanup()
		}
	})

	ginkgo.It("Use pod spec to create and delete embedded inline csi volumes", func() {
		client := f.ClientSet

		volSrc := &v1.VolumeSource{
			CSI: &v1.CSIVolumeSource{
				Driver: "csi-hostpath",
			},
		}

		hostpathPod, err := extractHostpathPod(client, f.Namespace.Name, "app=csi-hostpathplugin")
		framework.ExpectNoError(err)

		// deploy app pod
		pod := testCSIInlineVolumePod(f, volSrc)
		appPod, err := client.CoreV1().Pods(f.Namespace.Name).Create(pod)
		framework.ExpectNoError(err)
		time.Sleep(30 * time.Second)

		err = f.WaitForPodRunning(appPod.GetName())
		framework.ExpectNoError(err)

		// inject test file in volume
		_, err = utils.PodExec(appPod, "touch /data/helloworld")
		framework.ExpectNoError(err)
		resultStr, err := utils.PodExec(appPod, "/bin/ls")
		framework.ExpectNoError(err)
		if resultStr != "helloworld" {
			framework.Failf("unable to verify created file")
		}

		// validate file created by driver
		resultStr, err = utils.PodExec(hostpathPod, "find / -name helloworld")
		if !strings.Contains(resultStr, "helloworld") {
			framework.Failf("hostpath driver failed to create file in volume")
		}

		e2elog.Logf("Deleting pod %q/%q", appPod.Namespace, appPod.Name)
		framework.ExpectNoError(framework.DeletePodWithWait(f, client, appPod))
	})
})

func testCSIInlineVolumePod(f *framework.Framework, source *v1.VolumeSource) *v1.Pod {
	var (
		podName    = fmt.Sprintf("pod-inline-%x", rand.Uint32())
		volumeName = "test-vol-name"
		volumePath = "/data"
		image      = framework.BusyBoxImage
	)

	return &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: f.Namespace.Name,
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Name:  fmt.Sprintf("container-%x", rand.Uint32()),
					Image: image,
					Command: []string{
						"/bin/sh",
						"-c",
						"sleep 1000000",
					},
					VolumeMounts: []v1.VolumeMount{
						{
							Name:      volumeName,
							MountPath: volumePath,
						},
					},
				},
			},
			RestartPolicy: v1.RestartPolicyNever,
			Volumes: []v1.Volume{
				{
					Name:         volumeName,
					VolumeSource: *source,
				},
			},
		},
	}
}

func extractHostpathPod(client clientset.Interface, namespace, selector string) (*v1.Pod, error) {
	list, err := client.CoreV1().Pods(namespace).List(metav1.ListOptions{LabelSelector: selector})
	if err != nil {
		return nil, err
	}
	if len(list.Items) > 0 {
		return &list.Items[0], nil
	}
	return nil, fmt.Errorf("unable to find hospath plugin pod")
}
