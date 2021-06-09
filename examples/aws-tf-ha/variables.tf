
variable "controller_count" {
  type    = number
  default = 3
}
variable "worker_count" {
  type    = number
  default = 1
}

variable "region" {
  type    = string
  default = "eu-north-1"
}

variable "k0s_version" {
  type    = string
  default = "v1.21.1+k0s.0"
}