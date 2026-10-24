# Post images. Private: admins upload with signed PUT URLs and the site
# build downloads with signed GET URLs, both made by the API.
resource "google_storage_bucket" "images" {
  name     = "${var.project_id}-images"
  location = var.region # us-central1 is in the free tier

  uniform_bucket_level_access = true
  public_access_prevention    = "enforced"

  # The admin page uploads straight from the browser.
  cors {
    origin          = var.allowed_origins
    method          = ["PUT"]
    response_header = ["Content-Type", "x-goog-content-length-range"]
    max_age_seconds = 600
  }

  depends_on = [google_project_service.enabled]
}

# Signed URLs carry the signer's rights, so the API needs to read and create objects.
resource "google_storage_bucket_iam_member" "api_objects" {
  for_each = toset(["roles/storage.objectViewer", "roles/storage.objectCreator"])
  bucket   = google_storage_bucket.images.name
  role     = each.value
  member   = "serviceAccount:${google_service_account.api.email}"
}

# Lets the API sign URLs as itself through the IAM Credentials API, with no key file.
resource "google_service_account_iam_member" "api_signs_as_itself" {
  service_account_id = google_service_account.api.name
  role               = "roles/iam.serviceAccountTokenCreator"
  member             = "serviceAccount:${google_service_account.api.email}"
}
