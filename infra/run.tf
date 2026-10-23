resource "google_service_account" "api" {
    account_id = "api-runtime"
    display_name = "Cloud Run runtime for HGN bowls api"
}