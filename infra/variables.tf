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
  description = "Auth0 API identifier the admin tokens are issued for. Can't be changed once created."
}
variable "auth0_admin_client_id" {
  type        = string
  description = "Client ID of the admin SPA created in the Auth0 dashboard (public, not a secret)"
}
variable "google_oauth_client_id" {
  type        = string
  description = "Client ID of the club's Google OAuth client, for Google sign-in"
}
variable "google_oauth_client_secret" {
  type        = string
  description = "Secret of the club's Google OAuth client"
  sensitive   = true
  ephemeral   = true # only feeds a write-only attribute, so it never reaches state or plans
}
variable "cloudflare_account_id" {
  type = string
}
variable "pages_project" {
  type        = string
  description = "Cloudflare Pages project name"
}
