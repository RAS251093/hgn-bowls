variable "project_id" { type = string }
variable "region" {
  type    = string
  default = "us-central1"
}
variable "billing_account" {
  type        = string
  description = "Billing account ID"
}