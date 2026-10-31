# Inviting admins from the admin page. The API creates users on the admins
# connection, gives them club-admin and takes it away again, through the
# Auth0 Management API. Only holders of admin-manager can ask it to.
#
# Apply in stages. Cloud Run refuses a revision that reads a secret with
# no versions, so a plain apply would fail on the service before the secret
# has a value.
#
# 1. Create the Auth0 pieces, the secret and its IAM, but not the service:
#
#   op run --env-file=.env.op -- terraform apply \
#     -target=auth0_resource_server_scopes.api \
#     -target=auth0_role_permissions.admin_manager \
#     -target=auth0_client_grant.admins_manager \
#     -target='google_secret_manager_secret_iam_member.api_reads["AUTH0_MGMT_CLIENT_SECRET"]'
#
#    (Targets bring their dependencies: the role, the app and the secret.)
#
# 2. Copy the secret of "HGN Bowls API (manages admins)" from the Auth0
#    dashboard (Applications > the app > Settings > Client Secret) into the
#    1Password item hgn-bowls/auth0 as the field mgmt_client_secret, then:
#
#   op read -n op://hgn-bowls/auth0/mgmt_client_secret | gcloud secrets versions add auth0-mgmt-client-secret --data-file=-
#
# 3. Set up the email provider by hand (its API key stays out of state):
#    Resend > add and verify houghgreenparkbowls.uk (its DNS records go in
#    Cloudflare) > create a sending-only API key, saved in 1Password. Then
#    Auth0 > Branding > Email Provider > SMTP: host smtp.resend.com, port
#    587, username resend, password the API key, from
#    admin@houghgreenparkbowls.uk. Send Auth0's test email.
#    Until this is done, the next step fails on auth0_email_template
#    (branding.tf): Auth0 only lets templates change with its own provider
#    replaced.
#
# 4. Apply the rest, which gives the service its new env and styles the
#    sign-in pages and the email:
#
#   op run --env-file=.env.op -- terraform apply
#
# 5. In the Auth0 dashboard, User Management > Users > yourself > Roles,
#    assign admin-manager. Nobody else gets it from Terraform.
#
# Terraform never sees the app's secret: there is no auth0_client_credentials
# for it, and auth0_client doesn't keep the secret in state.

resource "auth0_role" "admin_manager" {
  name        = "admin-manager"
  description = "Committee members who invite admins"
}

resource "auth0_role_permissions" "admin_manager" {
  role_id = auth0_role.admin_manager.id

  permissions {
    name                       = "manage:admins"
    resource_server_identifier = auth0_resource_server.api.identifier
  }

  depends_on = [auth0_resource_server_scopes.api]
}

# The API signs in as this app to call the Management API. It needs no
# connection enabled: Management API calls aren't connection-scoped, and the
# set-password email goes out through the admin page's client, which is.
resource "auth0_client" "admins_manager" {
  name            = "HGN Bowls API (manages admins)"
  app_type        = "non_interactive"
  is_first_party  = true
  oidc_conformant = true
  grant_types     = ["client_credentials"]

  jwt_configuration {
    alg = "RS256"
  }
}

# Just enough to find and create users and move them in and out of roles.
# No delete:users: removing an admin takes away the role, not the account.
resource "auth0_client_grant" "admins_manager" {
  client_id = auth0_client.admins_manager.client_id
  audience  = "https://${var.auth0_domain}/api/v2/"
  scopes = [
    "read:users",
    "create:users",
    "update:users",
    "read:roles",
    "read:role_members",
    "create:role_members",
    "delete:role_members",
  ]
}
