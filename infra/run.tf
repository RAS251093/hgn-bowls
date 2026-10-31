resource "google_service_account" "api" {
  account_id   = "api-runtime"
  display_name = "Cloud Run runtime for HGN bowls api"
}

locals {
  # Browsers may call the API from the site, any Pages preview of it (each
  # gets its own subdomain) and local dev. The admin page signs in from the
  # same places (auth0.tf), and uploads to the bucket from anywhere (storage.tf).
  api_origins = concat(var.allowed_origins, [
    "https://*.${var.pages_project}.pages.dev",
    "http://localhost:4321",
  ])

  # Plain settings; secrets come from local.api_secrets in secrets.tf.
  api_env = {
    GOOGLE_CLOUD_PROJECT  = var.project_id
    CALENDAR_ID           = var.calendar_id
    ALLOWED_ORIGINS       = join(",", local.api_origins)
    TURSO_URL             = var.turso_url
    AUTH0_DOMAIN          = var.auth0_domain
    AUTH0_AUDIENCE        = var.auth0_audience
    AUTH0_CUSTOM_DOMAIN   = auth0_custom_domain.auth.domain
    BUCKET                = google_storage_bucket.images.name
    CLOUDFLARE_ACCOUNT_ID = var.cloudflare_account_id
    PAGES_PROJECT         = var.pages_project

    # For inviting admins (admins.tf). The API answers 503 on those routes
    # until these and AUTH0_MGMT_CLIENT_SECRET are all set.
    AUTH0_MGMT_CLIENT_ID  = auth0_client.admins_manager.client_id
    AUTH0_ADMIN_ROLE_ID   = auth0_role.club_admin.id
    AUTH0_ADMIN_CLIENT_ID = var.auth0_admin_client_id # sends the set-password email
  }
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
      image = "us-docker.pkg.dev/cloudrun/container/hello" # only used on first create, deploys own the image (see ignore_changes)

      resources {
        cpu_idle = true
        limits = {
          cpu    = "1"
          memory = "256Mi"
        }
      }

      dynamic "env" {
        for_each = local.api_env
        content {
          name  = env.key
          value = env.value
        }
      }

      dynamic "env" {
        for_each = local.api_secrets
        content {
          name = env.key
          value_source {
            secret_key_ref {
              secret  = google_secret_manager_secret.api[env.key].secret_id
              version = "latest"
            }
          }
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

  depends_on = [
    google_project_service.enabled,
    google_secret_manager_secret_iam_member.api_reads,
    # Accept tokens from the custom domain only once Auth0 has verified it.
    auth0_custom_domain_verification.auth,
  ]
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

# Runs "api migrate" against Turso. The deploy workflow points it at the new
# image and executes it before deploying the service.
resource "google_cloud_run_v2_job" "migrate" {
  name     = "migrate"
  location = var.region

  template {
    task_count = 1
    template {
      service_account = google_service_account.api.email
      max_retries     = 0
      timeout         = "120s"

      containers {
        image = "us-docker.pkg.dev/cloudrun/container/job" # deploys own the image (see ignore_changes)
        args  = ["migrate"]

        # Jobs always have CPU allocated, and Cloud Run then needs at least
        # 512Mi. It runs for seconds per deploy, well inside the free tier.
        resources {
          limits = {
            cpu    = "1"
            memory = "512Mi"
          }
        }

        env {
          name  = "TURSO_URL"
          value = var.turso_url
        }
        env {
          name = "TURSO_TOKEN"
          value_source {
            secret_key_ref {
              secret  = google_secret_manager_secret.api["TURSO_TOKEN"].secret_id
              version = "latest"
            }
          }
        }
      }
    }
  }

  lifecycle {
    ignore_changes = [
      template[0].template[0].containers[0].image,
      client,
      client_version,
    ]
  }

  depends_on = [
    google_project_service.enabled,
    google_secret_manager_secret_iam_member.api_reads,
  ]
}
