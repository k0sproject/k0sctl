terraform {
  required_providers {
      hcloud = {
          source = "hetznercloud/hcloud"
          version = "~> 1.24"
      }
  }
}
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

resource "hcloud_load_balancer" "load_balancer" {
  name       = "${var.cluster_name}-balancer"
  load_balancer_type = "lb11"
  location   = var.location
}

resource "hcloud_load_balancer_target" "load_balancer_target" {
  type             = "label_selector"
  load_balancer_id = hcloud_load_balancer.load_balancer.id
  label_selector = "role=controller"
}

resource "hcloud_load_balancer_service" "load_balancer_service_6443" {
    load_balancer_id = hcloud_load_balancer.load_balancer.id
    protocol = "tcp"
    listen_port = 6443
    destination_port = 6443
}

resource "hcloud_load_balancer_service" "load_balancer_service_9443" {
    load_balancer_id = hcloud_load_balancer.load_balancer.id
    protocol = "tcp"
    listen_port = 9443
    destination_port = 9443
}

resource "hcloud_load_balancer_service" "load_balancer_service_8132" {
    load_balancer_id = hcloud_load_balancer.load_balancer.id
    protocol = "tcp"
    listen_port = 8132
    destination_port = 8132
}

resource "hcloud_load_balancer_service" "load_balancer_service_8133" {
    load_balancer_id = hcloud_load_balancer.load_balancer.id
    protocol = "tcp"
    listen_port = 8133
    destination_port = 8133
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
                version = "0.11.1"
                "config" = {
                    "apiVersion" = "k0s.k0sproject.io/v1beta1"
                    "kind" =  "Cluster"
                    "metadata" = {
                        "name" = var.cluster_name
                    }
                    "spec" = {
                        "api" = {
                            "externalAddress" = hcloud_load_balancer.load_balancer.ipv4
                            "sans" = [hcloud_load_balancer.load_balancer.ipv4]
                        }
                    }
                }
            }
        }
    }
}

output "k0s_cluster" {
    value = yamlencode(local.k0s_tmpl)

}
