# Auth0 on the club's own domain, auth.houghgreenparkbowls.uk: sign-in and
# the set-password links in emails show the club's address, not Auth0's.
# Auth0 issues the certificate. Its CNAME (the custom_domain_cname output)
# goes in Cloudflare as DNS only: Auth0 can't issue the certificate through
# Cloudflare's proxy. Verification waits until that record resolves.

resource "auth0_custom_domain" "auth" {
  domain = "auth.houghgreenparkbowls.uk"
  type   = "auth0_managed_certs"
}

resource "auth0_custom_domain_verification" "auth" {
  custom_domain_id = auth0_custom_domain.auth.id

  timeouts {
    create = "15m"
  }
}

output "custom_domain_cname" {
  description = "The CNAME record Auth0 needs for auth.houghgreenparkbowls.uk (DNS only in Cloudflare)."
  value = {
    name   = auth0_custom_domain.auth.domain
    target = try(auth0_custom_domain.auth.verification[0].methods[0].record, null)
  }
}
