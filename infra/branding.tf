# The Auth0 pages admins see (sign in, choose a password) and the email
# that invites them, styled with the site's design system
# (ui/design-system/tokens.json) so they look like the club's own.
#
# The logo and font load from the live site, which lets Auth0's domain
# fetch fonts (ui/public/_headers).

locals {
  site = "https://houghgreenparkbowls.uk"

  # Design tokens used below.
  token = {
    background       = "#F3F6F4"
    card             = "#FBFCFB"
    foreground       = "#14211A"
    muted_foreground = "#4A5A50"
    primary          = "#1E3A2A"
    primary_hover    = "#2C5240"
    border           = "#D5DED8"
    input            = "#6F7F75"
    destructive      = "#9B2C1F"
  }
}

resource "auth0_branding" "admin" {
  logo_url    = "${local.site}/logos/hgb-roundel-green.svg"
  favicon_url = "${local.site}/logos/hgb-roundel-green.svg"

  colors {
    primary         = local.token.primary
    page_background = local.token.background
  }
}

# Flat and green-led like the site: no shadow, 8px controls, a 12px card.
resource "auth0_branding_theme" "admin" {
  display_name = "Hough Green Park Bowls Club"

  borders {
    button_border_radius = 8
    button_border_weight = 1
    buttons_style        = "rounded"
    input_border_radius  = 8
    input_border_weight  = 1
    inputs_style         = "rounded"
    show_widget_shadow   = false
    widget_border_weight = 1
    widget_corner_radius = 12
  }

  colors {
    primary_button            = local.token.primary
    primary_button_label      = local.token.background
    secondary_button_border   = local.token.primary
    secondary_button_label    = local.token.primary
    links_focused_components  = local.token.primary
    base_focus_color          = local.token.primary
    base_hover_color          = local.token.primary_hover
    header                    = local.token.primary
    body_text                 = local.token.foreground
    icons                     = local.token.muted_foreground
    input_background          = local.token.card
    input_border              = local.token.input
    input_filled_text         = local.token.foreground
    input_labels_placeholders = local.token.muted_foreground
    widget_background         = local.token.card
    widget_border             = local.token.border
    error                     = local.token.destructive
    success                   = local.token.primary_hover
    captcha_widget_theme      = "light"
  }

  # Atkinson Hyperlegible Next, at the site's 18px body size.
  fonts {
    font_url            = "${local.site}/fonts/atkinson-hyperlegible-next-latin-400-normal.woff2"
    links_style         = "underlined"
    reference_text_size = 18

    body_text {
      bold = false
      size = 100
    }
    buttons_text {
      bold = true
      size = 100
    }
    input_labels {
      bold = false
      size = 100
    }
    links {
      bold = true
      size = 100
    }
    subtitle {
      bold = false
      size = 100
    }
    title {
      bold = true
      size = 150
    }
  }

  page_background {
    background_color     = local.token.background
    background_image_url = ""
    page_layout          = "center"
  }

  widget {
    header_text_alignment = "center"
    logo_height           = 56
    logo_position         = "center"
    logo_url              = "${local.site}/logos/hgb-roundel-green.svg"
    social_buttons_layout = "bottom"
  }
}

# Plain words on the screens admins see, instead of Auth0's defaults.
resource "auth0_prompt_custom_text" "login_id" {
  prompt   = "login-id"
  language = "en"
  body = jsonencode({
    "login-id" = {
      title       = "Club admin"
      description = "Sign in to write and publish the club's news."
    }
  })
}

resource "auth0_prompt_custom_text" "login_password" {
  prompt   = "login-password"
  language = "en"
  body = jsonencode({
    "login-password" = {
      title       = "Enter your password"
      description = "For the club website's admin area."
    }
  })
}

# Invited admins land here from their email, so it reads as setting up,
# not as a reset.
resource "auth0_prompt_custom_text" "reset_password" {
  prompt   = "reset-password"
  language = "en"
  body = jsonencode({
    "reset-password" = {
      title       = "Choose your password"
      description = "For the club website's admin area. Use at least 12 characters."
    }
    "reset-password-success" = {
      title       = "Password saved"
      description = "You can now sign in to the club website's admin area."
    }
  })
}

# The email Auth0 sends for "change password" requests: both invitations
# (api/admins) and "Forgot password?". Auth0 only allows editing templates
# with its own email provider replaced, so apply this after the SMTP
# provider (Resend) is set up in the dashboard (see admins.tf).
resource "auth0_email_template" "reset_email" {
  template                = "reset_email"
  from                    = "Hough Green Park Bowls Club <admin@houghgreenparkbowls.uk>"
  subject                 = "Choose your password for the club website"
  syntax                  = "liquid"
  result_url              = "${local.site}/admin"
  url_lifetime_in_seconds = 604800 # 7 days: invitations sometimes wait a while
  enabled                 = true
  body                    = templatefile("${path.module}/emails/reset-email.html", { token = local.token, site = local.site })
}
