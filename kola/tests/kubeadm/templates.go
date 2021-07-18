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

var (
	workerConfig = `systemd:
  units:
    - name: prepare-cni-plugins.service
      enabled: true
      contents: |
        [Unit]
        Description=Unpack CNI plugins to /opt/cni/bin
        ConditionPathExists=!/opt/cni/bin
        [Service]
        Type=oneshot
        RemainAfterExit=true
        Restart=on-failure
        Environment=CNI_VERSION={{ .CNIVersion }}
        ExecStartPre=/usr/bin/mkdir --parents /opt/cni/bin
        ExecStartPre=/usr/bin/tar -v --extract --file "/opt/cni-plugins-linux-{{ .Arch }}-${CNI_VERSION}.tgz" --directory /opt/cni/bin --no-same-owner
        ExecStart=/usr/bin/rm "/opt/cni-plugins-linux-{{ .Arch }}-${CNI_VERSION}.tgz"
        [Install]
        WantedBy=multi-user.target
    - name: prepare-critools.service
      enabled: true
      contents: |
        [Unit]
        Description=Unpack CRI tools to /opt/bin
        ConditionPathExists=!/opt/bin/crictl
        [Service]
        Type=oneshot
        RemainAfterExit=true
        Restart=on-failure
        Environment=CRICTL_VERSION={{ .CRIctlVersion }}
        Environment=DOWNLOAD_DIR={{ .DownloadDir}}
        ExecStartPre=/usr/bin/mkdir --parents "${DOWNLOAD_DIR}"
        ExecStartPre=/usr/bin/tar -v --extract --file "/opt/crictl-${CRICTL_VERSION}-linux-{{ .Arch }}.tar.gz" --directory "${DOWNLOAD_DIR}" --no-same-owner
        ExecStart=/usr/bin/rm "/opt/crictl-${CRICTL_VERSION}-linux-{{ .Arch }}.tar.gz"
        [Install]
        WantedBy=multi-user.target
storage:
  files:
    - path: /opt/cni-plugins-linux-{{ .Arch }}-{{ .CNIVersion }}.tgz
      filesystem: root
      mode: 0644
      contents:
        remote:
          url: https://github.com/containernetworking/plugins/releases/download/{{ .CNIVersion }}/cni-plugins-linux-{{ .Arch }}-{{ .CNIVersion }}.tgz
          verification:
            hash:
              function: sha512
              sum: {{ .CNISum }}
    - path: /opt/crictl-{{ .CRIctlVersion }}-linux-{{ .Arch }}.tar.gz
      filesystem: root
      mode: 0644
      contents:
        remote:
          url: https://github.com/kubernetes-sigs/cri-tools/releases/download/{{ .CRIctlVersion }}/crictl-{{ .CRIctlVersion }}-linux-{{ .Arch }}.tar.gz
          verification:
            hash:
              function: sha512
              sum: {{ .CRIctlSum }}
    - path: {{ .DownloadDir }}/kubeadm
      filesystem: root
      mode: 0755
      contents:
        remote:
          url: https://storage.googleapis.com/kubernetes-release/release/{{ .Release }}/bin/linux/{{ .Arch }}/kubeadm
          verification:
            hash:
              function: sha512
              sum: {{ .KubeadmSum }}
    - path: {{ .DownloadDir }}/kubelet
      filesystem: root
      mode: 0755
      contents:
        remote:
          url: https://storage.googleapis.com/kubernetes-release/release/{{ .Release }}/bin/linux/{{ .Arch }}/kubelet
          verification:
            hash:
              function: sha512
              sum: {{ .KubeletSum }}
    - path: /home/core/install.sh
      filesystem: root
      mode: 0755
      contents:
        remote:
          url: "data:text/plain;base64,{{ .WorkerScript }}"`

	masterConfig = `systemd:
  units:
    - name: prepare-cni-plugins.service
      enabled: true
      contents: |
        [Unit]
        Description=Unpack CNI plugins to /opt/cni/bin
        ConditionPathExists=!/opt/cni/bin
        [Service]
        Type=oneshot
        RemainAfterExit=true
        Restart=on-failure
        Environment=CNI_VERSION={{ .CNIVersion }}
        ExecStartPre=/usr/bin/mkdir --parents /opt/cni/bin
        ExecStartPre=/usr/bin/tar -v --extract --file "/opt/cni-plugins-linux-{{ .Arch }}-${CNI_VERSION}.tgz" --directory /opt/cni/bin --no-same-owner
        ExecStart=/usr/bin/rm "/opt/cni-plugins-linux-{{ .Arch }}-${CNI_VERSION}.tgz"
        [Install]
        WantedBy=multi-user.target
    - name: prepare-critools.service
      enabled: true
      contents: |
        [Unit]
        Description=Unpack CRI tools to /opt/bin
        ConditionPathExists=!/opt/bin/crictl
        [Service]
        Type=oneshot
        RemainAfterExit=true
        Restart=on-failure
        Environment=CRICTL_VERSION={{ .CRIctlVersion }}
        Environment=DOWNLOAD_DIR={{ .DownloadDir}}
        ExecStartPre=/usr/bin/mkdir --parents "${DOWNLOAD_DIR}"
        ExecStartPre=/usr/bin/tar -v --extract --file "/opt/crictl-${CRICTL_VERSION}-linux-{{ .Arch }}.tar.gz" --directory "${DOWNLOAD_DIR}" --no-same-owner
        ExecStart=/usr/bin/rm "/opt/crictl-${CRICTL_VERSION}-linux-{{ .Arch }}.tar.gz"
        [Install]
        WantedBy=multi-user.target
storage:
  files:
    - path: /opt/cni-plugins-linux-{{ .Arch }}-{{ .CNIVersion }}.tgz
      filesystem: root
      mode: 0644
      contents:
        remote:
          url: https://github.com/containernetworking/plugins/releases/download/{{ .CNIVersion }}/cni-plugins-linux-{{ .Arch }}-{{ .CNIVersion }}.tgz
          verification:
            hash:
              function: sha512
              sum: {{ .CNISum }}
    - path: /opt/crictl-{{ .CRIctlVersion }}-linux-{{ .Arch }}.tar.gz
      filesystem: root
      mode: 0644
      contents:
        remote:
          url: https://github.com/kubernetes-sigs/cri-tools/releases/download/{{ .CRIctlVersion }}/crictl-{{ .CRIctlVersion }}-linux-{{ .Arch }}.tar.gz
          verification:
            hash:
              function: sha512
              sum: {{ .CRIctlSum }}
    - path: {{ .DownloadDir }}/kubeadm
      filesystem: root
      mode: 0755
      contents:
        remote:
          url: https://storage.googleapis.com/kubernetes-release/release/{{ .Release }}/bin/linux/{{ .Arch }}/kubeadm
          verification:
            hash:
              function: sha512
              sum: {{ .KubeadmSum }}
    - path: {{ .DownloadDir }}/kubelet
      filesystem: root
      mode: 0755
      contents:
        remote:
          url: https://storage.googleapis.com/kubernetes-release/release/{{ .Release }}/bin/linux/{{ .Arch }}/kubelet
          verification:
            hash:
              function: sha512
              sum: {{ .KubeletSum }}
    - path: {{ .DownloadDir }}/kubectl
      filesystem: root
      mode: 0755
      contents:
        remote:
          url: https://storage.googleapis.com/kubernetes-release/release/{{ .Release }}/bin/linux/{{ .Arch }}/kubectl
          verification:
            hash:
              function: sha512
              sum: {{ .KubectlSum }}
{{ if eq .CNI "cilium" }}
    - path: {{ .DownloadDir }}/cilium.tar.gz
      filesystem: root
      mode: 0755
      contents:
        remote:
          url: https://github.com/cilium/cilium-cli/releases/download/{{ .CiliumVersion }}/cilium-linux-amd64.tar.gz
{{ end }}
    - path: /home/core/install.sh
      filesystem: root
      mode: 0755
      contents:
        remote:
          url: "data:text/plain;base64,{{ .MasterScript }}"
    - path: /home/core/nginx.yaml
      filesystem: root
      mode: 0644
      contents:
        inline: |
          apiVersion: apps/v1
          kind: Deployment
          metadata:
            name: nginx-deployment
            labels:
              app: nginx
          spec:
            replicas: 1
            selector:
              matchLabels:
                app: nginx
            template:
              metadata:
                labels:
                  app: nginx
              spec:
                containers:
                - name: nginx
                  image: nginx
                  ports:
                  - containerPort: 80`

	masterScript = `#!/bin/bash
set -euo pipefail

export RELEASE_VERSION={{ .ReleaseVersion }}
export DOWNLOAD_DIR={{ .DownloadDir }}
export PATH="${PATH}:${DOWNLOAD_DIR}"

# create the required directory
mkdir --parent \
    /etc/systemd/system/kubelet.service.d \
    ${HOME}/.kube \
    /home/core/.kube

# we download and install the various requirements:
# * kubelet service and kubeadm dropin
    
curl --retry-delay 1 \
    --retry 60 \
    --retry-connrefused \
    --retry-max-time 60 \
    --connect-timeout 20 \
    --fail \
    -sSL \
    "https://raw.githubusercontent.com/kubernetes/release/${RELEASE_VERSION}/cmd/kubepkg/templates/latest/deb/kubelet/lib/systemd/system/kubelet.service" |
    sed "s:/usr/bin:${DOWNLOAD_DIR}:g" > /etc/systemd/system/kubelet.service
    
curl --retry-delay 1 \
    --retry 60 \
    --retry-connrefused \
    --retry-max-time 60 \
    --connect-timeout 20 \
    --fail \
    -sSL \
    "https://raw.githubusercontent.com/kubernetes/release/${RELEASE_VERSION}/cmd/kubepkg/templates/latest/deb/kubeadm/10-kubeadm.conf" |
    sed "s:/usr/bin:${DOWNLOAD_DIR}:g" > /etc/systemd/system/kubelet.service.d/10-kubeadm.conf
    
# we create the kubeadm config
# plugin-volume-dir and flex-volume-plugin-dir are required since /usr is read-only mounted
# etcd is also defined as external. The provided one has some issues with docker and selinux
# (permission denied with /var/lib/etcd) so it can't boot properly
cat << EOF > kubeadm-config.yaml
apiVersion: kubeadm.k8s.io/v1beta2
kind: InitConfiguration
nodeRegistration:
  kubeletExtraArgs:
    volume-plugin-dir: "/opt/libexec/kubernetes/kubelet-plugins/volume/exec/"
---
apiVersion: kubeadm.k8s.io/v1beta2
kind: ClusterConfiguration
networking:
  podSubnet: {{ .PodSubnet }}
controllerManager:
  extraArgs:
    flex-volume-plugin-dir: "/opt/libexec/kubernetes/kubelet-plugins/volume/exec/"
etcd:
  external:
    endpoints:
    {{ range $endpoint := .Endpoints }}
      - {{ $endpoint }}
    {{ end }}
EOF

{{ if eq .CNI "calico" }}
cat << EOF > calico.yaml
# Source: https://docs.projectcalico.org/manifests/custom-resources.yaml
apiVersion: operator.tigera.io/v1
kind: Installation
metadata:
  name: default
spec:
  # Configures Calico networking.
  calicoNetwork:
    # Note: The ipPools section cannot be modified post-install.
    ipPools:
      - blockSize: 26
        cidr: {{ .PodSubnet }}
        encapsulation: VXLANCrossSubnet
        natOutgoing: Enabled
        nodeSelector: all()
  flexVolumePath: /opt/libexec/kubernetes/kubelet-plugins/volume/exec/
EOF
{{ end }}

{
    systemctl enable --quiet --now kubelet
    kubeadm config images pull
    kubeadm init --config kubeadm-config.yaml
    cp /etc/kubernetes/admin.conf $HOME/.kube/config
    cp /etc/kubernetes/admin.conf /home/core/.kube/config
    chown -R core:core /home/core/.kube; chmod a+r /home/core/.kube/config;

{{ if eq .CNI "calico" }}
    kubectl create -f https://docs.projectcalico.org/manifests/tigera-operator.yaml
    kubectl apply -f calico.yaml
{{ end }}
{{ if eq .CNI "flannel" }}
    curl -sSL https://raw.githubusercontent.com/flannel-io/flannel/master/Documentation/kube-flannel.yml > kube-flannel.yml
    sed -i "s#10.244.0.0/16#{{ .PodSubnet }}#" kube-flannel.yml
    kubectl apply -f kube-flannel.yml
{{ end }}
{{ if eq .CNI "cilium" }}
    sudo tar -xf {{ .DownloadDir }}/cilium.tar.gz -C {{ .DownloadDir }}
    /opt/bin/cilium install \
        --config enable-endpoint-routes=true \
        --config cluster-pool-ipv4-cidr={{ .PodSubnet }}
    # --wait will wait for status to report success
    /opt/bin/cilium status --wait
{{ end }}
} 1>&2


URL=$(kubectl config view -o jsonpath='{.clusters[0].cluster.server}')
prefix="https://"
short_url=${URL#"${prefix}"}
token=$(kubeadm token create)
certHashes=$(openssl x509 -pubkey -in /etc/kubernetes/pki/ca.crt | openssl rsa -pubin -outform der 2>/dev/null | openssl dgst -sha256 -hex | sed 's/^.* //')

cat << EOF
apiVersion: kubeadm.k8s.io/v1beta2
kind: JoinConfiguration
discovery:
  bootstrapToken:
    apiServerEndpoint: ${short_url}
    token: ${token}
    caCertHashes:
    - sha256:${certHashes}
controlPlane:
nodeRegistration:
  kubeletExtraArgs:
    volume-plugin-dir: "/opt/libexec/kubernetes/kubelet-plugins/volume/exec/"
EOF
`

	workerScript = `#!/bin/bash
set -euo pipefail

export RELEASE_VERSION={{ .ReleaseVersion }}
export DOWNLOAD_DIR={{ .DownloadDir }}
export PATH="${PATH}:${DOWNLOAD_DIR}"

# create the required directory
mkdir --parent \
    /opt/cni/bin \
    /etc/systemd/system/kubelet.service.d

# we download and install the various requirements
# * kubelet service and kubeadm dropin

curl --retry-delay 1 \
    --retry 60 \
    --retry-connrefused \
    --retry-max-time 60 \
    --connect-timeout 20 \
    --fail \
    -sSL \
    "https://raw.githubusercontent.com/kubernetes/release/${RELEASE_VERSION}/cmd/kubepkg/templates/latest/deb/kubelet/lib/systemd/system/kubelet.service" |
    sed "s:/usr/bin:${DOWNLOAD_DIR}:g" |
    tee /etc/systemd/system/kubelet.service

curl --retry-delay 1 \
    --retry 60 \
    --retry-connrefused \
    --retry-max-time 60 \
    --connect-timeout 20 \
    --fail \
    -sSL \
    "https://raw.githubusercontent.com/kubernetes/release/${RELEASE_VERSION}/cmd/kubepkg/templates/latest/deb/kubeadm/10-kubeadm.conf" |
    sed "s:/usr/bin:${DOWNLOAD_DIR}:g" |
    tee /etc/systemd/system/kubelet.service.d/10-kubeadm.conf

systemctl enable --now kubelet

cat << EOF > worker-config.yaml
{{ .WorkerConfig }}
EOF

systemctl start --quiet coreos-metadata
ipv4=$(cat /run/metadata/flatcar | grep -v IPV6 | grep IP | grep -E '(PRIVATE|LOCAL|DYNAMIC)' | cut -d = -f 2)

kubeadm join --config worker-config.yaml --node-name "${ipv4}"`
)
