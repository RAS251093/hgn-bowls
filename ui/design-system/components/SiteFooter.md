# SiteFooter

`src/components/ui/SiteFooter.astro` closes every page on club green with the club name, contact details and secondary links.

```astro
<SiteFooter email="secretary@example.org" phone="0151 000 0000" />
```

**Props:** `email` and `phone`, shown as bracketed placeholders until the committee confirms them.
- Text is `text-primary-muted-foreground`; links stay underlined and at least 44px tall.
- No newsletter sign-up, social icon rows or back-to-top buttons.
