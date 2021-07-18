#!/bin/bash
set -euo pipefail

export RELEASE_VERSION=v0.4.0
export DOWNLOAD_DIR=/opt/bin
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
  podSubnet: 192.168.0.0/17
controllerManager:
  extraArgs:
    flex-volume-plugin-dir: "/opt/libexec/kubernetes/kubelet-plugins/volume/exec/"
etcd:
  external:
    endpoints:
    
      - http://1.2.3.4:2379
    
EOF



{
    systemctl enable --quiet --now kubelet
    kubeadm config images pull
    kubeadm init --config kubeadm-config.yaml
    cp /etc/kubernetes/admin.conf $HOME/.kube/config
    cp /etc/kubernetes/admin.conf /home/core/.kube/config
    chown -R core:core /home/core/.kube; chmod a+r /home/core/.kube/config;



    curl -sSL https://raw.githubusercontent.com/flannel-io/flannel/master/Documentation/kube-flannel.yml > kube-flannel.yml
    sed -i "s#10.244.0.0/16#192.168.0.0/17#" kube-flannel.yml
    kubectl apply -f kube-flannel.yml


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
