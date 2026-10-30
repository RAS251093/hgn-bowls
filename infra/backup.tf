# Weekly backup of every post (drafts and deleted ones too) to a private
# bucket. The repo is public, so backups never go to GitHub. A Cloud Run job
# runs "api backup" every Sunday morning.
#
# To restore, pick an export and load it into an empty database (a local
# file to check it, or a fresh Turso database to recover):
#   gcloud storage ls gs://hgn-bowls-backups/posts/
#   gcloud storage cp gs://hgn-bowls-backups/posts/<name>.json backup.json
#   cd api && TURSO_URL=file:restored.db go run . restore ../backup.json
# Cover images aren't in the export: they stay in the images bucket, which
# the restored rows point at.

resource "google_storage_bucket" "backups" {
  name     = "${var.project_id}-backups"
  location = var.region # free tier

  uniform_bucket_level_access = true
  public_access_prevention    = "enforced"

  # Thirteen weekly copies are plenty, and keep it well inside the free 5 GB.
  lifecycle_rule {
    condition {
      age = 90
    }
    action {
      type = "Delete"
    }
  }

  depends_on = [google_project_service.enabled]
}

# The job's own identity: it can read the database token and add backups,
# nothing else (not overwrite or delete them).
resource "google_service_account" "backup" {
  account_id   = "backup"
  display_name = "Weekly posts backup"
}

resource "google_secret_manager_secret_iam_member" "backup_reads_turso" {
  secret_id = google_secret_manager_secret.api["TURSO_TOKEN"].id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${google_service_account.backup.email}"
}

resource "google_storage_bucket_iam_member" "backup_writes" {
  bucket = google_storage_bucket.backups.name
  role   = "roles/storage.objectCreator"
  member = "serviceAccount:${google_service_account.backup.email}"
}

resource "google_cloud_run_v2_job" "backup" {
  name     = "backup"
  location = var.region

  template {
    task_count = 1
    template {
      service_account = google_service_account.backup.email
      max_retries     = 1
      timeout         = "300s"

      containers {
        image = "us-docker.pkg.dev/cloudrun/container/job" # deploys own the image (see ignore_changes)
        args  = ["backup"]

        # As for migrate: jobs need at least 512Mi, and this runs for
        # seconds a week, well inside the free tier.
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
          name  = "BACKUP_BUCKET"
          value = google_storage_bucket.backups.name
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
    google_secret_manager_secret_iam_member.backup_reads_turso,
  ]
}

# Cloud Scheduler starts the job every Sunday at 03:17 UK time (three
# scheduler jobs are free per billing account; this is the only one).
resource "google_service_account" "backup_scheduler" {
  account_id   = "backup-scheduler"
  display_name = "Starts the weekly backup"
}

resource "google_cloud_run_v2_job_iam_member" "scheduler_runs_backup" {
  name     = google_cloud_run_v2_job.backup.name
  location = google_cloud_run_v2_job.backup.location
  role     = "roles/run.invoker"
  member   = "serviceAccount:${google_service_account.backup_scheduler.email}"
}

resource "google_cloud_scheduler_job" "backup" {
  name      = "weekly-backup"
  region    = var.region
  schedule  = "17 3 * * 0"
  time_zone = "Europe/London"

  http_target {
    http_method = "POST"
    uri         = "https://run.googleapis.com/v2/${google_cloud_run_v2_job.backup.id}:run"

    oauth_token {
      service_account_email = google_service_account.backup_scheduler.email
      scope                 = "https://www.googleapis.com/auth/cloud-platform"
    }
  }

  depends_on = [
    google_project_service.enabled,
    google_cloud_run_v2_job_iam_member.scheduler_runs_backup,
  ]
}

# An email if a backup run fails (after its one retry).
resource "google_monitoring_alert_policy" "backup_failed" {
  display_name = "Weekly backup failed"
  combiner     = "OR"

  conditions {
    display_name = "backup job execution failed"
    condition_threshold {
      filter          = "metric.type=\"run.googleapis.com/job/completed_execution_count\" AND resource.type=\"cloud_run_job\" AND resource.label.job_name=\"${google_cloud_run_v2_job.backup.name}\" AND metric.label.result=\"failed\""
      comparison      = "COMPARISON_GT"
      threshold_value = 0
      duration        = "0s"

      aggregations {
        alignment_period   = "300s"
        per_series_aligner = "ALIGN_SUM"
      }
    }
  }

  notification_channels = [google_monitoring_notification_channel.email.id]
}

# Deploys point the job at each new image, as they do for migrate.
resource "google_cloud_run_v2_job_iam_member" "deployer_backup" {
  name     = google_cloud_run_v2_job.backup.name
  location = google_cloud_run_v2_job.backup.location
  role     = "roles/run.developer"
  member   = "serviceAccount:${google_service_account.deployer.email}"
}

resource "google_service_account_iam_member" "deployer_actas_backup" {
  service_account_id = google_service_account.backup.name
  role               = "roles/iam.serviceAccountUser"
  member             = "serviceAccount:${google_service_account.deployer.email}"
}

output "backup_bucket" {
  value = google_storage_bucket.backups.name
}
