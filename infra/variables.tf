variable "project_id" { type = string }
variable "region" {
  type    = string
  default = "us-central1"
}
variable "billing_account" {
  type        = string
  description = "Billing account ID"
}
variable "alert_email" {
  type        = string
  description = "Email address for api alerts"
}