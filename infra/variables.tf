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
variable "calendar_id" {
  type        = string
  description = "Public Google Calendar the events come from"
}
variable "allowed_origins" {
  type        = list(string)
  description = "Website origins allowed to call the API and upload images"
}
variable "turso_url" {
  type        = string
  description = "libsql:// URL of the Turso database"
}
variable "auth0_domain" {
  type        = string
  description = "Auth0 tenant domain, for example club.eu.auth0.com"
}
variable "auth0_audience" {
  type        = string
  description = "Auth0 API identifier the admin tokens are issued for"
}
variable "cloudflare_account_id" {
  type = string
}
variable "pages_project" {
  type        = string
  description = "Cloudflare Pages project name"
}
