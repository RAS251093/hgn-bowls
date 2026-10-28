# SiteHeader

`src/components/ui/SiteHeader.astro` is the club green bar at the top of every public page: the roundel and wordmark on the left, the main nav on the right.

```astro
<SiteHeader current="What's on" />
```

**Props:** `current`, the nav label of the page you're on (`Home`, `What's on`, `News`, `Find us`, `Join us`), which gets `aria-current="page"` and a turf underline.
- Nav labels are fixed and sentence case. Use the same words in links, headings and the footer.
- The roundel is `/logos/hgb-roundel-light.svg` at 44px with empty alt; the link text names the club.
- Put Hero straight after it so the two read as one green block.
- Below 768px the nav wraps under the brand; every link keeps a 44px target.
- The admin area uses the same bar with "Club admin" as the wordmark, the signed-in name in `text-primary-muted-foreground`, View website and an `outline` `sm` Sign out button.
