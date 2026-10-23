data "google_project" "this" {}

resource "google_billing_budget" "monthly" {
  billing_account = var.billing_account
  display_name    = "hgn-bowls monthly"

  budget_filter {
    projects = ["projects/${data.google_project.this.number}"]
  }

  amount {
    specified_amount {
      currency_code = "GBP"
      units         = "1"
    }
  }

  threshold_rules { threshold_percent = 0.5 } # £0.50 spent
  threshold_rules { threshold_percent = 1.0 } # £1 spent
  threshold_rules {
    threshold_percent = 1.0
    spend_basis       = "FORECASTED_SPEND" # predicted > £1
  }

  depends_on = [google_project_service.enabled]
}