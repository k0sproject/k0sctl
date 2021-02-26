variable "hcloud_token" {
    description = "Hetzner API token"
}

provider "hcloud" {
  token = var.hcloud_token
}

variable "ssh_keys" {
    default = []
}

variable "ssh_user" {
    default = "root"
}

variable "cluster_name" {
    default = "k0s"
}

variable "location" {
    default = "hel1"
}

variable "image" {
    default = "ubuntu-18.04"
}

variable "controller_type" {
    default = "cx31"
}

variable "controller_count" {
    default = 1
}

variable "worker_count" {
    default = 1
}

variable "worker_type" {
    default = "cx31"
}

resource "hcloud_server" "controller" {
    count = var.controller_count
    name = "${var.cluster_name}-controller-${count.index}"
    image = var.image
    server_type = var.controller_type
    ssh_keys = var.ssh_keys
    location = var.location
    labels = {
        role = "controller"
    }

}

resource "hcloud_server" "worker" {
    count = var.worker_count
    name = "${var.cluster_name}-worker-${count.index}"
    image = var.image
    server_type = var.worker_type
    ssh_keys = var.ssh_keys
    location = var.location
    labels = {
        role = "worker"
    }
}

locals {
    k0s_tmpl = {
        apiVersion = "k0sctl.k0sproject.io/v1beta1"
        kind = "cluster"
        spec = {
            hosts = [
                for host in concat(hcloud_server.controller, hcloud_server.worker) : {
                    ssh = {
                        address = host.ipv4_address
                        user = "root"
                    }
                    role = host.labels.role
                }
            ]
            k0s = {
                version = "0.10.0-beta2"
            }
        }
    }
}

output "k0s_cluster" {
    value = yamlencode(local.k0s_tmpl)

}
