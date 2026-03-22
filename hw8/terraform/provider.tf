terraform {
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 6.7.0"
    }
    docker = {
      source  = "kreuzwerker/docker"
      version = "~> 2.0"
    }
  }
}

provider "aws" {
  region = var.aws_region
}

data "aws_ecr_authorization_token" "registry" {}

provider "docker" {
  registry_auth {
    address  = "704722031841.dkr.ecr.us-west-2.amazonaws.com"
    username = "AWS"
    password = data.aws_ecr_authorization_token.registry.password
  }
}