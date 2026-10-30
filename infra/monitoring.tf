resource "google_monitoring_notification_channel" "email" {
  display_name = "Alerts email"
  type         = "email"
  labels = {
    email_address = var.alert_email
  }
  depends_on = [google_project_service.enabled]
}

resource "google_monitoring_uptime_check_config" "api_health" {
  display_name = "api /health"
  timeout      = "10s"
  period       = "900s" # every 15 minutes

  http_check {
    path         = "/health"
    port         = 443
    use_ssl      = true
    validate_ssl = true
  }

  monitored_resource {
    type = "uptime_url"
    labels = {
      project_id = var.project_id
      host       = trimprefix(google_cloud_run_v2_service.api.uri, "https://")
    }
  }

  depends_on = [google_project_service.enabled]
}

resource "google_monitoring_alert_policy" "api_down" {
  display_name = "API is down"
  combiner     = "OR"

  conditions {
    display_name = "/health failing from more than one location"
    condition_threshold {
      filter          = "metric.type=\"monitoring.googleapis.com/uptime_check/check_passed\" AND resource.type=\"uptime_url\" AND metric.label.check_id=\"${google_monitoring_uptime_check_config.api_health.uptime_check_id}\""
      comparison      = "COMPARISON_GT"
      threshold_value = 1
      duration        = "900s"

      aggregations {
        alignment_period     = "1200s"
        per_series_aligner   = "ALIGN_NEXT_OLDER"
        cross_series_reducer = "REDUCE_COUNT_FALSE"
        group_by_fields      = ["resource.label.host"]
      }
    }
  }

  notification_channels = [google_monitoring_notification_channel.email.id]
}
# The public site, on its own domain (Cloudflare Pages). The page must load
# and still be the club's site, not an error page with a 200.
resource "google_monitoring_uptime_check_config" "site_home" {
  display_name = "site home page"
  timeout      = "10s"
  period       = "900s" # every 15 minutes

  http_check {
    path         = "/"
    port         = 443
    use_ssl      = true
    validate_ssl = true
  }

  content_matchers {
    content = "Hough Green Park Bowls Club"
    matcher = "CONTAINS_STRING"
  }

  monitored_resource {
    type = "uptime_url"
    labels = {
      project_id = var.project_id
      host       = "houghgreenparkbowls.uk"
    }
  }

  depends_on = [google_project_service.enabled]
}

resource "google_monitoring_alert_policy" "site_down" {
  display_name = "Website is down"
  combiner     = "OR"

  conditions {
    display_name = "home page failing from more than one location"
    condition_threshold {
      filter          = "metric.type=\"monitoring.googleapis.com/uptime_check/check_passed\" AND resource.type=\"uptime_url\" AND metric.label.check_id=\"${google_monitoring_uptime_check_config.site_home.uptime_check_id}\""
      comparison      = "COMPARISON_GT"
      threshold_value = 1
      duration        = "900s"

      aggregations {
        alignment_period     = "1200s"
        per_series_aligner   = "ALIGN_NEXT_OLDER"
        cross_series_reducer = "REDUCE_COUNT_FALSE"
        group_by_fields      = ["resource.label.host"]
      }
    }
  }

  notification_channels = [google_monitoring_notification_channel.email.id]
}
