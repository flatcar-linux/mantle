// Copyright 2021 Kinvolk GmbH
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
package kubeadm

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"strings"
	"text/template"
	"time"

	"github.com/coreos/pkg/capnslog"

	"github.com/coreos/mantle/kola"
	"github.com/coreos/mantle/kola/cluster"
	"github.com/coreos/mantle/kola/register"
	"github.com/coreos/mantle/kola/tests/etcd"
	"github.com/coreos/mantle/platform"
	"github.com/coreos/mantle/platform/conf"
	"github.com/coreos/mantle/util"
)

var (
	// CNIs is the list of CNIs to deploy
	// in the cluster setup
	CNIs = []string{
		"calico",
		"flannel",
		"cilium",
	}
	// params are used to render script templates
	// Release is the kubernetes release version we want to use
	// ReleaseVersion is the version of the kubelet service and kubeadm dropin
	// TODO: when a new version of kubernetes will be tested, it would be nice
	// to have a map[string]Release with Release struct holding the parameter below
	params = map[string]interface{}{
		// TODO: it's actually the CLI version
		// we should pass the CLI and cilium version
		// https://github.com/cilium/cilium-cli/issues/118
		"CiliumVersion":  "v0.8.3",
		"CNIVersion":     "v0.8.7",
		"CRIctlVersion":  "v1.17.0",
		"ReleaseVersion": "v0.4.0",
		"Release":        "v1.21.0",
		"DownloadDir":    "/opt/bin",
		"PodSubnet":      "192.168.0.0/17",
		"KubeadmSum":     "0673408403a3474c868ae86109f11f9114bca7ddce204be0d169316fb3ce0edefa4b2a472ba9b8308e423e6b927d4098ac36296405570f444f39551fb1c4bbb4",
		"KubeletSum":     "530689c0cc32ef1830f7ae26ac10995f815043d48a905141e23a34a5e61522c4ee2ff46953648c47c5592d7c2ffa40ce90469a697f36f68475b8da5abd73f9f5",
		"CRIctlSum":      "e258f4607a89b8d44c700036e636dd42cc3e2ed27a3bb13beef736f80f64f10b7974c01259a66131d3f7b44ed0c61b1ca0ea91597c416a9c095c432de5112d44",
		"CNISum":         "8f2cbee3b5f94d59f919054dccfe99a8e3db5473b553d91da8af4763e811138533e05df4dbeab16b3f774852b4184a7994968f5e036a3f531ad1ac4620d10ede",
		"KubectlSum":     "9557d298146ef62ffbcf05b3591bf1ce74f345628370447a4f614b5f64e367b5bfa8e397cc4755da9ea38f1ba04c95c65c313e735550ffc3b03c197e936c3e11",
	}
	plog       = capnslog.NewPackageLogger("github.com/coreos/mantle", "kola/tests/kubeadm")
	etcdConfig = conf.ContainerLinuxConfig(`
etcd:
  advertise_client_urls: http://{PRIVATE_IPV4}:2379
  listen_client_urls: http://0.0.0.0:2379
systemd:
  units:
    - name: etcd-member.service
      enabled: true
`)
)

func init() {
	for _, CNI := range CNIs {
		register.Register(&register.Test{
			Name:             fmt.Sprintf("kubeadm.%s.base", CNI),
			Distros:          []string{"cl"},
			ExcludePlatforms: []string{"esx"},
			Run: func(c cluster.TestCluster) {
				kubeadmBaseTest(c, CNI)
			},
		})
	}
}

// kubeadmBaseTest asserts that the cluster is up and running
func kubeadmBaseTest(c cluster.TestCluster, CNI string) {
	board := kola.QEMUOptions.Board
	params["Arch"] = strings.SplitN(board, "-", 2)[0]
	params["CNI"] = CNI
	kubectl, err := setup(c)
	if err != nil {
		c.Fatalf("unable to setup cluster: %v", err)
	}

	c.Run("node readiness", func(c cluster.TestCluster) {
		// we let some times to the cluster to be fully booted
		if err := util.Retry(10, 10*time.Second, func() error {
			// notice the extra space before "Ready", it's to not catch
			// "NotReady" nodes
			out := c.MustSSH(kubectl, "/opt/bin/kubectl get nodes | grep \" Ready\"| wc -l")
			readyNodesCnt := string(out)
			if readyNodesCnt != "2" {
				return fmt.Errorf("ready nodes should be equal to 2: %s", readyNodesCnt)
			}

			return nil
		}); err != nil {
			c.Fatalf("nodes are not ready: %v", err)
		}
	})
	c.Run("nginx deployment", func(c cluster.TestCluster) {
		// nginx manifest has been deployed through ignition
		if _, err := c.SSH(kubectl, "/opt/bin/kubectl apply -f nginx.yaml"); err != nil {
			c.Fatalf("unable to deploy nginx: %v", err)
		}

		if err := util.Retry(10, 10*time.Second, func() error {
			out := c.MustSSH(kubectl, "/opt/bin/kubectl get deployments -o json | jq '.items | .[] | .status.readyReplicas'")
			readyCnt := string(out)
			if readyCnt != "1" {
				return fmt.Errorf("ready replicas should be equal to 1: %s", readyCnt)
			}

			return nil
		}); err != nil {
			c.Fatalf("nginx is not deployed: %v", err)
		}
	})
}

// render takes care of template rendering
// using `b` parameter, we can render in a base64 encoded format
func render(s string, p map[string]interface{}, b bool) (*bytes.Buffer, error) {
	tmpl, err := template.New("install").Parse(s)
	if err != nil {
		return nil, fmt.Errorf("unable to parse script: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, p); err != nil {
		return nil, fmt.Errorf("unable to execute template: %w", err)
	}

	if b {
		b64 := base64.StdEncoding.EncodeToString(buf.Bytes())
		buf.Reset()
		if _, err := buf.WriteString(b64); err != nil {
			return nil, fmt.Errorf("unable to write bas64 content to buffer: %w", err)
		}
	}

	return &buf, nil
}

// setup creates a cluster with kubeadm
// cluster is composed by etcd node, worker and master node
// it returns master node in order to have direct access on node
// with kubectl installed and setup
func setup(c cluster.TestCluster) (platform.Machine, error) {
	plog.Infof("creating etcd node")

	etcdNode, err := c.NewMachine(etcdConfig)
	if err != nil {
		return nil, fmt.Errorf("unable to create etcd node: %w", err)
	}

	if err := etcd.GetClusterHealth(c, etcdNode, 1); err != nil {
		return nil, fmt.Errorf("unable to get etcd node health: %w", err)
	}

	params["Endpoints"] = []string{fmt.Sprintf("http://%s:2379", etcdNode.PrivateIP())}

	plog.Infof("creating master node")

	mScript, err := render(masterScript, params, true)
	if err != nil {
		return nil, fmt.Errorf("unable to render master script: %w", err)
	}

	params["MasterScript"] = mScript.String()

	masterCfg, err := render(masterConfig, params, false)
	if err != nil {
		return nil, fmt.Errorf("unable to render container linux config for master: %w", err)
	}

	master, err := c.NewMachine(conf.ContainerLinuxConfig(masterCfg.String()))
	if err != nil {
		return nil, fmt.Errorf("unable to create master node: %w", err)
	}

	out, err := c.SSH(master, "sudo /home/core/install.sh")
	if err != nil {
		return nil, fmt.Errorf("unable to run master script: %w", err)
	}

	// "out" holds the worker config generated
	// by the master script install
	params["WorkerConfig"] = string(out)

	plog.Infof("creating worker node")
	wScript, err := render(workerScript, params, true)
	if err != nil {
		return nil, fmt.Errorf("unable to render worker script: %w", err)
	}

	params["WorkerScript"] = wScript.String()

	workerCfg, err := render(workerConfig, params, false)
	if err != nil {
		return nil, fmt.Errorf("unable to render container linux config for master: %w", err)
	}

	worker, err := c.NewMachine(conf.ContainerLinuxConfig(workerCfg.String()))
	if err != nil {
		return nil, fmt.Errorf("unable to create worker node: %w", err)
	}

	out, err = c.SSH(worker, "sudo /home/core/install.sh")
	if err != nil {
		return nil, fmt.Errorf("unable to run worker script: %w", err)
	}

	return master, nil
}
