resource "google_service_account" "api" {
  account_id   = "api-runtime"
  display_name = "Cloud Run runtime for HGN bowls api"
}

resource "google_cloud_run_v2_service" "api" {
  name     = "api"
  location = var.region
  ingress  = "INGRESS_TRAFFIC_ALL"

  template {
    service_account = google_service_account.api.email

    scaling {
      min_instance_count = 0
      max_instance_count = 2
    }

    containers {
      image = "us-docker.pkg.dev/cloudrun/container/hello" #placeholder

      resources {
        cpu_idle = true
        limits = {
          cpu    = "1"
          memory = "256Mi"
        }
      }
    }
  }

  lifecycle {
    # GHA deploys alter these
    ignore_changes = [
      template[0].containers[0].image,
      client,
      client_version,
    ]
  }

  depends_on = [google_project_service.enabled]
}

# API is public (website and astro build call it). Admin routes check jwt.
resource "google_cloud_run_v2_service_iam_member" "public" {
  name     = google_cloud_run_v2_service.api.name
  location = google_cloud_run_v2_service.api.location
  role     = "roles/run.invoker"
  member   = "allUsers"
}

output "api_url" {
  value = google_cloud_run_v2_service.api.uri
}
