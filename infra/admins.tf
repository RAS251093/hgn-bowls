# Inviting admins from the admin page. The API creates users on the admins
# connection, gives them club-admin and takes it away again, through the
# Auth0 Management API. Only holders of admin-manager can ask it to.
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

# The API signs in as this app to call the Management API. Auth0 only lets
# an app create users on a connection it's enabled for, so it's enabled on
# admins (auth0.tf); with only the client-credentials grant it still can't
# sign anyone in. The set-password email goes out through the admin page's
# client.
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
