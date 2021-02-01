# Bootstrapping a k0s cluster on AWS using Terraform

This directory provides an example flow with `k0sctl` tool together with Terraform using AWS as the cloud provider.

## Prerequisites
- You need an account and AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY, AWS_SESSION_TOKEN for AWS
- Terraform >=v0.14.3 installed
- You will need the `k0sctl` binary in `PATH` 

## TF Steps
- `terraform init`
- `terraform apply`
- `terraform output -raw k0s_cluster | k0sctl apply --config -` NOTE: this assumes that `k0sctl` binary is available in the `PATH`

This will create a cluster with single controller and worker nodes. 
If you want to override the default behaviour. Create a `terraform.tfvars` file with the needed details. You can use the provided `terraform.tfvars.example` as a template.

## Makefile steps

In case you don't want to do all those steps you can use the Makefile. 

To deploy a k0s cluster with k0sctl:
- `make apply` 

Get kubeconfig:
- `make kubeconfig`
Teardown:
- `make destroy`
