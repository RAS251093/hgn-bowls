# Admin sign-in. The tenant and the machine-to-machine app Terraform signs in
# with were created by hand; everything else here is managed.

locals {
  # Where the admin page can start sign-in from. Local dev uses Astro's port.
  admin_origins = concat(var.allowed_origins, ["http://localhost:4321"])
}

# The Go API. Tokens for it carry a permissions claim (RBAC) and last 10
# minutes, which bounds how long a token works after sign-out or role removal.
resource "auth0_resource_server" "api" {
  name       = "HGN Bowls API"
  identifier = var.auth0_audience

  signing_alg                                     = "RS256"
  enforce_policies                                = true
  token_dialect                                   = "access_token_authz"
  token_lifetime                                  = 600
  token_lifetime_for_web                          = 600
  allow_offline_access                            = true # refresh tokens for the admin page
  skip_consent_for_verifiable_first_party_clients = true
}

resource "auth0_resource_server_scopes" "api" {
  resource_server_identifier = auth0_resource_server.api.identifier

  scopes {
    name        = "manage:posts"
    description = "Write, publish and delete news posts"
  }
}

resource "auth0_role" "club_admin" {
  name        = "club-admin"
  description = "Committee members who manage the website"
}

resource "auth0_role_permissions" "club_admin" {
  role_id = auth0_role.club_admin.id

  permissions {
    name                       = "manage:posts"
    resource_server_identifier = auth0_resource_server.api.identifier
  }

  depends_on = [auth0_resource_server_scopes.api]
}

# The admin page, created by hand in the dashboard and adopted here.
import {
  to = auth0_client.admin
  id = var.auth0_admin_client_id
}

resource "auth0_client" "admin" {
  name            = "HGN Bowls admin"
  app_type        = "spa"
  is_first_party  = true
  oidc_conformant = true

  grant_types         = ["authorization_code", "refresh_token"]
  callbacks           = [for o in local.admin_origins : "${o}/admin"]
  allowed_logout_urls = [for o in local.admin_origins : "${o}/"]
  web_origins         = local.admin_origins

  jwt_configuration {
    alg = "RS256"
  }

  # Each refresh hands out a new token and retires the old one; reusing a
  # retired one makes Auth0 revoke the whole chain.
  refresh_token {
    rotation_type                = "rotating"
    expiration_type              = "expiring"
    leeway                       = 0
    idle_token_lifetime          = 86400  # 1 day unused
    token_lifetime               = 604800 # 7 days at most
    infinite_idle_token_lifetime = false
    infinite_token_lifetime      = false
  }
}

# Passkeys, with an emailed code for the first sign-in (and if a passkey is
# lost). No passwords, and nobody can sign themselves up: admins are invited.
resource "auth0_connection" "admins" {
  name     = "admins"
  strategy = "auth0"

  options {
    disable_signup         = true
    brute_force_protection = true

    authentication_methods {
      passkey {
        enabled = true
      }
      email_otp {
        enabled = true
      }
      password {
        enabled = false
      }
    }

    passkey_options {
      challenge_ui                   = "both"
      progressive_enrollment_enabled = true
      local_enrollment_enabled       = true
    }
  }
}

# Google sign-in with the club's own OAuth client (Auth0's shared dev keys
# aren't for production). The secret is write-only, so it never reaches state.
resource "auth0_connection" "google" {
  name     = "google-oauth2"
  strategy = "google-oauth2"

  options {
    client_id = var.google_oauth_client_id
    scopes    = ["email", "profile"]
  }

  options_client_secret_wo         = var.google_oauth_client_secret
  options_client_secret_wo_version = 1 # bump to rotate the secret
}

resource "auth0_connection_clients" "admins" {
  connection_id   = auth0_connection.admins.id
  enabled_clients = [auth0_client.admin.client_id]
}

resource "auth0_connection_clients" "google" {
  connection_id   = auth0_connection.google.id
  enabled_clients = [auth0_client.admin.client_id]
}

# Passkeys need the new Universal Login with the identifier asked first.
resource "auth0_prompt" "login" {
  universal_login_experience = "new"
  identifier_first           = true
}

output "auth0_admin_client_id" {
  value = auth0_client.admin.client_id
}
