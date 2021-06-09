provider "aws" {
  region = var.region
}

module "instances" {
  source           = "../aws-tf"
  controller_count = var.controller_count
  worker_count     = var.worker_count
}

data "aws_vpc" "default" {
  default = true
}

data "aws_subnet_ids" "all" {
  vpc_id = data.aws_vpc.default.id
}

module "elb" {
  source = "terraform-aws-modules/elb/aws"

  name = "elb"

  subnets         = data.aws_subnet_ids.all.ids
  security_groups = [module.instances.cluster-security_group]
  internal        = false

  listener = [
    {
      instance_port     = "6443"
      instance_protocol = "tcp"
      lb_port           = "6443"
      lb_protocol       = "tcp"
    },
    {
      instance_port     = "9443"
      instance_protocol = "tcp"
      lb_port           = "9443"
      lb_protocol       = "tcp"
    },
    {
      instance_port     = "8132"
      instance_protocol = "tcp"
      lb_port           = "8132"
      lb_protocol       = "tcp"
    },
    {
      instance_port     = "8133"
      instance_protocol = "tcp"
      lb_port           = "8133"
      lb_protocol       = "tcp"
    },
  ]

  health_check = {
    target              = "TCP:6443"
    interval            = 30
    healthy_threshold   = 2
    unhealthy_threshold = 2
    timeout             = 5
  }

  tags = {
    Owner       = "k0s"
    Environment = "terraform"
  }

  # ELB attachments
  number_of_instances = var.controller_count
  instances           = concat(module.instances.controller-instances.*.id)
}

output "elb" {
  value = module.elb
}
locals {
  k0s_tmpl = {
    apiVersion = "k0sctl.k0sproject.io/v1beta1"
    kind       = "cluster"
    spec = {
      hosts = [
        for host in concat(module.instances.controller-instances, module.instances.worker-instances) : {
          ssh = {
            address = host.public_ip
            user    = "ubuntu"
            keyPath = "../aws-tf/aws_private.pem"
           
          }
          role = host.tags["Name"]
        }
      ]
      k0s = {
        version = var.k0s_version
        "config" = {
          "apiVersion" = "k0s.k0sproject.io/v1beta1"
          "kind"       = "Cluster"
          "metadata" = {
            "name" = "ha-cluster"
          }
          "spec" = {
            "api" = {
              "externalAddress" = module.elb.elb_dns_name
              "sans"            = [module.elb.elb_dns_name]
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
