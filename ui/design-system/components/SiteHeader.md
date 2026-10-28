# SiteHeader

`src/components/ui/SiteHeader.astro` is the club green bar at the top of every public page: the roundel and wordmark on the left, the main nav on the right.

```astro
<SiteHeader current="What's on" />
```

**Props:** `current`, the nav label of the page you're on (`Home`, `What's on`, `News`, `Find us`), which gets `aria-current="page"` and a turf underline.
- Nav labels are fixed and sentence case. Use the same words in links, headings and the footer.
- The roundel is `/logos/hgb-roundel-light.svg` at 44px with empty alt; the link text names the club.
- Put Hero straight after it so the two read as one green block.
- Below 768px the nav wraps under the brand; every link keeps a 44px target.
- The admin area uses the same bar with "Club admin" as the wordmark, the signed-in name in `text-primary-muted-foreground`, View website and an `outline` `sm` Sign out button.

**On phones** (below 768px) the nav folds behind a Menu button so the header stays one row. The menu lists the links. Escape or choosing a link closes it. Without JavaScript the nav stays open, so nobody loses it.

There is no Join us button while the club has no online sign-up; it returns with the Join us page.
