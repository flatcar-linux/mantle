// Copyright 2016 CoreOS, Inc.
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
	"regexp"
	"strings"
	"text/template"
	"time"

	"github.com/coreos/mantle/kola/cluster"
	"github.com/coreos/mantle/kola/tests/etcd"
	"github.com/coreos/mantle/platform"
	"github.com/coreos/mantle/platform/conf"
	"github.com/coreos/mantle/util"
)

// kCluster just keeps track of which machines are which in a
// platform.TestCluster with kubernetes running.
type kCluster struct {
	etcd    platform.Machine
	master  platform.Machine
	workers []platform.Machine
}

// Setup a multi-node cluster based on generic scrips from coreos-kubernetes repo.
// https://github.com/coreos/coreos-kubernetes/tree/master/multi-node/generic
func setupCluster(c cluster.TestCluster, nodes int, version, runtime string) *kCluster {
	// start single-node etcd
	etcdNode, err := c.NewMachine(etcdConfig)
	if err != nil {
		c.Fatalf("error creating etcd: %v", err)
	}

	if err := etcd.GetClusterHealth(c, etcdNode, 1); err != nil {
		c.Fatalf("error checking etcd health: %v", err)
	}

	// passing cloud-config has the side effect of populating `/etc/environment`,
	// which the install script depends on
	master, err := c.NewMachine(conf.CloudConfig(""))
	if err != nil {
		c.Fatalf("error creating master: %v", err)
	}

	options := map[string]string{
		"HYPERKUBE_IMAGE_REPO": "quay.io/coreos/hyperkube",
		"MASTER_HOST":          master.PrivateIP(),
		"ETCD_ENDPOINTS":       fmt.Sprintf("http://%v:2379", etcdNode.PrivateIP()),
		"CONTROLLER_ENDPOINT":  fmt.Sprintf("https://%v:443", master.PrivateIP()),
		"K8S_SERVICE_IP":       "10.3.0.1",
		"K8S_VER":              version,
		"CONTAINER_RUNTIME":    runtime,
	}

	// generate TLS assets on master
	if err := generateMasterTLSAssets(c, master, options); err != nil {
		c.Fatalf("error creating master tls: %v", err)
	}

	// create worker nodes
	workers, err := platform.NewMachines(c, conf.CloudConfig(""), nodes)
	if err != nil {
		c.Fatalf("error creating workers: %v", err)
	}

	// generate tls assets on workers by transfering ca from master
	if err := generateWorkerTLSAssets(c, master, workers); err != nil {
		c.Fatalf("error creating worker tls: %v", err)
	}

	// configure nodes via generic install scripts
	runInstallScript(c, master, controllerInstallScript, options)

	for _, worker := range workers {
		runInstallScript(c, worker, workerInstallScript, options)
	}

	// configure kubectl
	if err := configureKubectl(c, master, master.PrivateIP(), version); err != nil {
		c.Fatalf("error configuring master kubectl: %v", err)
	}

	// check that all nodes appear in kubectl
	f := func() error {
		return nodeCheck(c, master, workers)
	}
	if err := util.Retry(15, 30*time.Second, f); err != nil {
		c.Fatalf("error waiting for nodes: %v", err)
	}

	cluster := &kCluster{
		etcd:    etcdNode,
		master:  master,
		workers: workers,
	}
	return cluster
}

func generateMasterTLSAssets(c cluster.TestCluster, master platform.Machine, options map[string]string) error {
	var buffer = new(bytes.Buffer)

	tmpl, err := template.New("masterCNF").Parse(masterCNF)
	if err != nil {
		return err
	}
	if err := tmpl.Execute(buffer, options); err != nil {
		return err
	}

	if err := platform.InstallFile(buffer, master, "/home/core/openssl.cnf"); err != nil {
		return err
	}

	var cmds = []string{
		// gen master assets
		"openssl genrsa -out ca-key.pem 2048",
		`openssl req -x509 -new -nodes -key ca-key.pem -days 10000 -out ca.pem -subj "/CN=kube-ca"`,
		"openssl genrsa -out apiserver-key.pem 2048",
		`openssl req -new -key apiserver-key.pem -out apiserver.csr -subj "/CN=kube-apiserver" -config openssl.cnf`,
		"openssl x509 -req -in apiserver.csr -CA ca.pem -CAkey ca-key.pem -CAcreateserial -out apiserver.pem -days 365 -extensions v3_req -extfile openssl.cnf",

		// gen cluster admin keypair
		"openssl genrsa -out admin-key.pem 2048",
		`openssl req -new -key admin-key.pem -out admin.csr -subj "/CN=kube-admin"`,
		"openssl x509 -req -in admin.csr -CA ca.pem -CAkey ca-key.pem -CAcreateserial -out admin.pem -days 365",

		// move into /etc/kubernetes/ssl
		"sudo mkdir -p /etc/kubernetes/ssl",
		"sudo cp /home/core/ca.pem /etc/kubernetes/ssl/ca.pem",
		"sudo cp /home/core/apiserver.pem /etc/kubernetes/ssl/apiserver.pem",
		"sudo cp /home/core/apiserver-key.pem /etc/kubernetes/ssl/apiserver-key.pem",
		"sudo chmod 600 /etc/kubernetes/ssl/*-key.pem",
		"sudo chown root:root /etc/kubernetes/ssl/*-key.pem",
	}

	for _, cmd := range cmds {
		b, err := c.SSH(master, cmd)
		if err != nil {
			return fmt.Errorf("Failed on cmd: %s with error: %s and output %s", cmd, err, b)
		}
	}
	return nil
}

func generateWorkerTLSAssets(c cluster.TestCluster, master platform.Machine, workers []platform.Machine) error {
	for i, worker := range workers {
		// copy tls assets from master to workers
		err := platform.TransferFile(master, "/etc/kubernetes/ssl/ca.pem", worker, "/home/core/ca.pem")
		if err != nil {
			return err
		}
		err = platform.TransferFile(master, "/home/core/ca-key.pem", worker, "/home/core/ca-key.pem")
		if err != nil {
			return err
		}

		// place worker-openssl.cnf on workers
		cnf := strings.Replace(workerCNF, "{{.WORKER_IP}}", worker.PrivateIP(), -1)
		in := strings.NewReader(cnf)
		if err := platform.InstallFile(in, worker, "/home/core/worker-openssl.cnf"); err != nil {
			return err
		}

		// gen certs
		workerFQDN := fmt.Sprintf("kube-worker-%v", i)
		cmds := []string{
			fmt.Sprintf("openssl genrsa -out worker-key.pem 2048"),
			fmt.Sprintf(`openssl req -new -key worker-key.pem -out %v-worker.csr -subj "/CN=%v" -config worker-openssl.cnf`, workerFQDN, workerFQDN),
			fmt.Sprintf(`openssl x509 -req -in %v-worker.csr -CA ca.pem -CAkey ca-key.pem -CAcreateserial -out worker.pem -days 365 -extensions v3_req -extfile worker-openssl.cnf`, workerFQDN),

			// move into /etc/kubernetes/ssl
			"sudo mkdir -p /etc/kubernetes/ssl",
			"sudo chmod 600 /home/core/*-key.pem",
			"sudo chown root:root /home/core/*-key.pem",
			"sudo cp /home/core/worker.pem /etc/kubernetes/ssl/worker.pem",
			"sudo cp /home/core/worker-key.pem /etc/kubernetes/ssl/worker-key.pem",
			"sudo cp /home/core/ca.pem /etc/kubernetes/ssl/ca.pem",
		}

		for _, cmd := range cmds {
			b, err := c.SSH(worker, cmd)
			if err != nil {
				return fmt.Errorf("Failed on cmd: %s with error: %s and output %s", cmd, err, b)
			}
		}
	}
	return nil
}

// https://coreos.com/kubernetes/docs/latest/configure-kubectl.html
func configureKubectl(c cluster.TestCluster, m platform.Machine, server string, version string) error {
	// ignore suffix like '-coreos.1' to grab upstream kubelet
	version, err := stripSemverSuffix(version)
	if err != nil {
		return err
	}

	var (
		ca        = "/home/core/ca.pem"
		adminKey  = "/home/core/admin-key.pem"
		adminCert = "/home/core/admin.pem"
		kubeURL   = fmt.Sprintf("https://storage.googleapis.com/kubernetes-release/release/%v/bin/linux/amd64/kubectl", version)
	)

	if _, err := c.SSH(m, "wget -q "+kubeURL); err != nil {
		return err
	}
	if _, err := c.SSH(m, "chmod +x ./kubectl"); err != nil {
		return err
	}

	// cmds to configure kubectl
	cmds := []string{
		fmt.Sprintf("./kubectl config set-cluster default-cluster --server=https://%v --certificate-authority=%v", server, ca),
		fmt.Sprintf("./kubectl config set-credentials default-admin --certificate-authority=%v --client-key=%v --client-certificate=%v", ca, adminKey, adminCert),
		"./kubectl config set-context default-system --cluster=default-cluster --user=default-admin",
		"./kubectl config use-context default-system",
	}
	for _, cmd := range cmds {
		b, err := c.SSH(m, cmd)
		if err != nil {
			return fmt.Errorf("Failed on cmd: %s with error: %s and output %s", cmd, err, b)
		}
	}
	return nil
}

var semverPrefix = regexp.MustCompile(`^v[\d]+\.[\d]+\.[\d]+`)

// Strip semver suffix -- e.g., v1.1.8_coreos.1 --> v1.1.8. If no match
// found, return error.
func stripSemverSuffix(v string) (string, error) {
	v = semverPrefix.FindString(v)
	if v == "" {
		return "", fmt.Errorf("error stripping semver suffix")
	}

	return v, nil
}

// Run and configure the coreos-kubernetes generic install scripts.
func runInstallScript(c cluster.TestCluster, m platform.Machine, script string, options map[string]string) {
	c.MustSSH(m, "sudo stat /usr/lib/flatcar/kubelet-wrapper")

	var buffer = new(bytes.Buffer)

	tmpl, err := template.New("installScript").Parse(script)
	if err != nil {
		c.Fatal(err)
	}
	if err := tmpl.Execute(buffer, options); err != nil {
		c.Fatal(err)
	}

	if err := platform.InstallFile(buffer, m, "/home/core/install.sh"); err != nil {
		c.Fatal(err)
	}

	// use client to collect stderr
	client, err := m.SSHClient()
	if err != nil {
		c.Fatal(err)
	}
	defer client.Close()
	session, err := client.NewSession()
	if err != nil {
		c.Fatal(err)
	}
	defer session.Close()

	stderr := bytes.NewBuffer(nil)
	session.Stderr = stderr

	err = session.Start("sudo /home/core/install.sh")
	if err != nil {
		c.Fatal(err)
	}

	// timeout script to prevent it looping forever
	errc := make(chan error)
	go func() {
		errc <- session.Wait()
	}()
	select {
	case err := <-errc:
		if err != nil {
			c.Fatal(err)
		}
	case <-time.After(time.Minute * 7):
		c.Fatal("Timed out waiting for install script to finish.")
	}
}

var (
	etcdConfig = conf.ContainerLinuxConfig(`
etcd:
  advertise_client_urls: http://{PUBLIC_IPV4}:2379
  listen_client_urls: http://0.0.0.0:2379
systemd:
  units:
    - name: etcd-member.service
      enabled: true
`)
)

const (
	masterCNF = `[req]
req_extensions = v3_req
distinguished_name = req_distinguished_name
[req_distinguished_name]
[ v3_req ]
basicConstraints = CA:FALSE
keyUsage = nonRepudiation, digitalSignature, keyEncipherment
subjectAltName = @alt_names
[alt_names]
DNS.1 = kubernetes
DNS.2 = kubernetes.default
IP.1 = {{.K8S_SERVICE_IP}}
IP.2 = {{.MASTER_HOST}}`

	workerCNF = `[req]
req_extensions = v3_req
distinguished_name = req_distinguished_name
[req_distinguished_name]
[ v3_req ]
basicConstraints = CA:FALSE
keyUsage = nonRepudiation, digitalSignature, keyEncipherment
subjectAltName = @alt_names
[alt_names]
IP.1 = {{.WORKER_IP}}`
)
