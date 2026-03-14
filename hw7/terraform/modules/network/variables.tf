variable "service_name" {
  type = string
}

variable "container_port" {
  type = number
}

variable "aws_region" {
  type    = string
  default = "us-west-2"
}