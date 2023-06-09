locals {
  # You have the choice of setting your Hetzner API token here or define the TF_VAR_hcloud_token env
  # within your shell, such as such: export TF_VAR_hcloud_token=xxxxxxxxxxx 
  # If you choose to define it in the shell, this can be left as is.

  # Your Hetzner token can be found in your Project > Security > API Token (Read & Write is required).
  hcloud_token = "xxxxxxxxxxx"
}

module "kube-hetzner" {
  source = "kube-hetzner/kube-hetzner/hcloud"
  providers = {
    hcloud = hcloud
  }
  hcloud_token = var.hcloud_token != "" ? var.hcloud_token : local.hcloud_token

  ssh_public_key = file("~/.ssh/id_ed25519.pub")
  ssh_private_key = file("~/.ssh/id_ed25519")

  network_region = "eu-central"

  control_plane_nodepools = [
    {
      name        = "control-plane",
      server_type = "cax21",
      location    = "fsn1",
      labels      = [],
      taints      = [],
      count       = 3
    },
  ]

  agent_nodepools = [
    {
      name        = "agent",
      server_type = "cax21",
      location    = "fsn1",
      labels      = [],
      taints      = [],
      count       = 0
    }
  ]

  create_kustomization = false

  enable_longhorn = true
  longhorn_repository = "https://charts.longhorn.io"
  longhorn_namespace = "longhorn-system"
  longhorn_fstype = "ext4"
  longhorn_replica_count = 1

  enable_klipper_metal_lb = true
  allow_scheduling_on_control_plane = true
  automatically_upgrade_k3s = false
  automatically_upgrade_os = false

  # Longhorn, all Longhorn helm values can be found at https://github.com/longhorn/longhorn/blob/master/chart/values.yaml
  # The following is an example, please note that the current indentation inside the EOT is important.
  longhorn_values = <<EOT
defaultSettings:
  defaultDataPath: /var/longhorn
persistence:
  defaultFsType: ext4
  defaultClassReplicaCount: 1
  defaultClass: true
  EOT
}

provider "hcloud" {
  token = var.hcloud_token != "" ? var.hcloud_token : local.hcloud_token
}

terraform {
  required_version = ">= 1.3.3"
  required_providers {
    hcloud = {
      source  = "hetznercloud/hcloud"
      version = ">= 1.39.0"
    }
  }
}

output "kubeconfig" {
  value     = module.kube-hetzner.kubeconfig
  sensitive = true
}

variable "hcloud_token" {
  sensitive = true
  default   = ""
}
