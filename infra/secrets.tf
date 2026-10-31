# Terraform only creates the secrets. Their values are added outside
# Terraform so they never reach state, for example:
#   op read op://hgn-bowls/api/turso_token | gcloud secrets versions add turso-token --data-file=-
# Cloud Run won't start a revision whose secret has no version yet, so a new
# secret needs a targeted apply (the secret and its IAM) and a value before
# the full apply.
locals {
  api_secrets = {
    TURSO_TOKEN          = "turso-token"
    CALENDAR_API_KEY     = "calendar-api-key"
    PAGES_DEPLOY_HOOK    = "pages-deploy-hook"
    CLOUDFLARE_API_TOKEN = "cloudflare-api-token"
    BUILD_TOKEN          = "build-token"

    # The admins app's secret (admins.tf), copied from the Auth0 dashboard.
    AUTH0_MGMT_CLIENT_SECRET = "auth0-mgmt-client-secret"
  }
}

resource "google_secret_manager_secret" "api" {
  for_each  = local.api_secrets
  secret_id = each.value

  replication {
    auto {}
  }

  depends_on = [google_project_service.enabled]
}

resource "google_secret_manager_secret_iam_member" "api_reads" {
  for_each  = google_secret_manager_secret.api
  secret_id = each.value.id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${google_service_account.api.email}"
}
