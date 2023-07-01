# Comet Server Kubernetes Charts (Experimental)

Deploy a Comet Server in Hetzner Cloud. Built using Hetzner's k3s terraform provider - [kube-hetzner](https://github.com/kube-hetzner/terraform-hcloud-kube-hetzner).

### Charts

|Type|Description|Chart
|---|---|---
|Standalone|Deploy a standalone Comet Server|[charts/comet-server](./charts/comet-server/)
|Operator (WIP)|Deploy a Comet Server operator with built-in admin UI|[charts/comet-server-operator](./charts/comet-server-operator/)

### Deploy a test cluster in Hetzner Cloud

**How it works:**

1. Use [packer](https://www.packer.io/) to create a Hetzner snapshot containing the base k3s node image.
2. Use [terraform](https://www.terraform.io/) to create a configure a three-node k3s cluster using Hetzner Cloud VMs.
3. Use a [comet-server](./charts/comet-server) helm chart to deploy [ghcr.io/cometbackup/comet-server](https://github.com/cometbackup/comet-server-docker/pkgs/container/comet-server).
4. Use pre-configured `account.cometbackup.com` credentials to generate a Self-Hosted Comet Server serials.

**Requirements:**

Please ensure that you have the `hcloud` CLI, `terraform`, `packer`, `kubectl`, and `helm` installed before continuing.

**Usage:**

Bring up the cluster -

```bash
export HCLOUD_TOKEN="<your-hcloud-token>"
export TF_VAR_hcloud_token=$HCLOUD_TOKEN

# Build the node images -
# This only needs to be run once to create the initial images
packer init packer/hcloud-microos-snapshots.pkr.hcl
packer build packer/hcloud-microos-snapshots.pkr.hcl

# Bring up the cluster
terraform init --upgrade
terraform apply --auto-approve

# You should now have a kubeconfig file in the project root -
# Export this, and test the cluster connection
export KUBECONFIG=/k3s_kubeconfig.yaml
kubectl get nodes -o wide
```

Add the Helm repo and deploy [charts/comet-server](./charts/comet-server/) - 

```bash
kubectl create secret generic comet-api-token \
  --namespace default \
  --from-literal email=<email> \
  --from-literal token=<token>

helm repo add comet-k8s https://benjamesfleming.github.io/comet-k8s

helm install cometd comet-k8s/comet-server
```