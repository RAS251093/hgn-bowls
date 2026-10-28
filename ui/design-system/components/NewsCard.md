# NewsCard

`src/components/ui/NewsCard.astro` is a club news teaser: cover photo, date, headline and the post's short description.

```astro
<div class="grid gap-8 sm:grid-cols-2 lg:grid-cols-3">
  <NewsCard href="/news/open-day" title="Open day this Saturday" date="Saturday 12 April" dateTime="2027-04-12" image={cover} description="Come along and try bowls" />
</div>
```

**Props:** `href`, `title`, `date` (written out) and `dateTime`, `image` (`ImageMetadata`, or a remote URL downloaded at build time; decorative here because the headline names the post), an optional `description` and `level` (2 or 3, default 3). A post without its own cover shows the club photo of two bowls on the green.
- The whole card is clickable through the headline link; only the headline is in the tab order.
- Cards sit on a `bg-secondary` band; the card is `bg-card` with a 1px border and `rounded-lg` corners.
- The date is `text-meta` in `text-meta-foreground`, written out ("Saturday 12 April"), never tracked capitals.
- Hover darkens the edge to `border-input` and underlines the headline. No lift, no shadow.
- Home shows the latest 3; the news list shows all, newest first.
