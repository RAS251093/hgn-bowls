# NewsCard

`src/components/ui/NewsCard.astro` is a club news teaser: cover photo, date, headline and the post's short description.

```astro
<div class="grid gap-8 sm:grid-cols-2 lg:grid-cols-3">
  <NewsCard href="/news/open-day" title="Open day this Saturday" date={post.data.pubDate} image={post.data.heroImage} description={post.data.description} />
</div>
```

**Props:** `href`, `title`, `date`, `image` (`ImageMetadata`, decorative here because the headline names the post) and an optional `description`.
- The whole card is clickable through the headline link; only the headline is in the tab order.
- Cards sit on a `bg-secondary` band; the card is `bg-card` with a 1px border and `rounded-lg` corners.
- The date is `text-meta` in `text-meta-foreground`, written out ("Saturday 12 April"), never tracked capitals.
- Hover darkens the edge to `border-input` and underlines the headline. No lift, no shadow.
- Home shows the latest 3; the news list shows all, newest first.
