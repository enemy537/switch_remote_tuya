variable "project" {
  type    = string
  default = "switch-remote"
}

variable "region" {
  type    = string
  default = "us-west-2"
}

variable "database_url" {
  type      = string
  sensitive = true
}

variable "tuya_client_id" {
  type      = string
  sensitive = true
}

variable "tuya_client_secret" {
  type      = string
  sensitive = true
}

variable "jwt_secret" {
  type      = string
  sensitive = true
}

variable "tuya_base_url" {
  type    = string
  default = "https://openapi.tuyaus.com"
}
