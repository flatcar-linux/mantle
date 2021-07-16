// Copyright 2021 Kinvolk GmbH
// Copyright 2015 CoreOS, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package kubernetes

import (
	"bytes"
	"fmt"
	"strings"
	"time"

	"github.com/coreos/mantle/kola/cluster"
	"github.com/coreos/mantle/kola/register"
	"github.com/coreos/mantle/platform"
	"github.com/coreos/mantle/util"
)

// register a separate test for each version tag
var basicTags = []string{
	"v1.14.10",
	"v1.16.8",
	"v1.18.0",
}

// regester each tag once per runtime
var runtimes = []string{
	"docker",
}

func init() {
	for i := range basicTags {
		for j := range runtimes {
			// use closure to store version and runtime in a Test
			t, r := basicTags[i], runtimes[j]
			f := func(c cluster.TestCluster) {
				CoreOSBasic(c, t, r)
			}

			register.Register(&register.Test{
				Name:        "google.kubernetes.basic." + r + "." + t,
				Run:         f,
				ClusterSize: 0,
				Platforms:   []string{"gce", "do", "aws", "qemu", "azure"}, // TODO: fix packet, esx
				Distros:     []string{"cl"},
				// incompatible with docker >=20.10
				ExcludeChannels: []string{"alpha"},
			})
		}
	}
}

// Run basic smoke tests on cluster. Assumes master is machine index 1,
// workers make up the rest.
func CoreOSBasic(c cluster.TestCluster, version, runtime string) {
	// only one worker node to run on VMware which has max 3 machines for one test currently (the other two are one for etcd and one controller)
	k := setupCluster(c, 1, version, runtime)

	// start nginx pod and curl endpoint
	if err := nginxCheck(c, k.master, k.workers); err != nil {
		c.Fatal(err)
	}
}

func nodeCheck(c cluster.TestCluster, master platform.Machine, nodes []platform.Machine) error {
	b, err := c.SSH(master, "./kubectl get nodes")
	if err != nil {
		return err
	}

	// parse kubectl output for IPs
	addrMap := map[string]struct{}{}
	for _, line := range strings.Split(string(b), "\n")[1:] {
		addrMap[strings.SplitN(line, " ", 2)[0]] = struct{}{}
	}

	// add master to node list because it is now normal to register
	// master nodes but have it set as unschedulable in kubernetes v1.2+
	nodes = append(nodes, master)

	if len(addrMap) != len(nodes) {
		return fmt.Errorf("cannot detect all nodes in kubectl output \n%v\n%v", addrMap, nodes)
	}
	for _, node := range nodes {
		if _, ok := addrMap[node.PrivateIP()]; !ok {
			return fmt.Errorf("node IP missing from kubectl get nodes")
		}
	}
	return nil
}

func nginxCheck(c cluster.TestCluster, master platform.Machine, nodes []platform.Machine) error {
	pod := strings.NewReader(nginxPodYAML)
	secret := strings.NewReader(secretYAML)
	if err := platform.InstallFile(pod, master, "./nginx-pod.yaml"); err != nil {
		return err
	}
	if err := platform.InstallFile(secret, master, "./secret.yaml"); err != nil {
		return err
	}

	if _, err := c.SSH(master, "./kubectl create -f secret.yaml"); err != nil {
		return err
	}

	if _, err := c.SSH(master, "./kubectl create -f nginx-pod.yaml"); err != nil {
		return err
	}
	// wait for pod status to be 'Running'
	podIsRunning := func() error {
		b, err := c.SSH(master, "./kubectl get pod nginx --template={{.status.phase}}")
		if err != nil {
			return err
		}
		if !bytes.Equal(b, []byte("Running")) {
			return fmt.Errorf("nginx pod not running: %s", b)
		}
		return nil
	}
	if err := util.Retry(10, 10*time.Second, podIsRunning); err != nil {
		return err
	}

	// delete pod
	_, err := c.SSH(master, "./kubectl delete pods nginx")
	if err != nil {
		return err
	}

	return nil
}

const (
	secretYAML = `apiVersion: v1
kind: Secret
metadata:
  name: test-secret
data:
  data-1: dmFsdWUtMQ0K
  data-2: dmFsdWUtMg0KDQo=`

	nginxPodYAML = `apiVersion: v1
kind: Pod
metadata:
  name: nginx
  labels:
    app: nginx
spec:
  containers:
  - name: nginx
    image: nginx
    ports:
    - containerPort: 80
    volumeMounts:
      # name must match the volume name below
      - name: secret-volume
        mountPath: /etc/secret-volume
  volumes:
    - name: secret-volume
      secret:
        secretName: test-secret`
)
