# Hero

`src/components/ui/Hero.astro` is the club green band that opens each page, with the page's one `h1`.

```astro
<Hero title="Bowls in the heart of Hough Green Park" lede="A friendly crown green club…" image={walking} imageAlt="Two club members walking up the green">
  <Fragment slot="actions">
    <Button variant="accent" href="/join">Come and try it</Button>
    <Button variant="outline" href="/whats-on">What's on</Button>
  </Fragment>
</Hero>
```

**Props:** `title`, `lede`, `image` (an imported `ImageMetadata`) and `imageAlt`. The `actions` slot takes one `accent` Button and at most one `outline`.
- It sits straight under SiteHeader, so the two read as one green block.
- The heading is `text-heading-hero` (48px), stepping down to `text-heading-xl` on phones. No eyebrow line above it.
- The photo has `rounded-lg` corners and no frame, overlay or gradient.
- Inner pages may drop the photo and the actions; keep the heading and lede.
- This is the one big colour moment on a page. Don't add another green band until the footer.
