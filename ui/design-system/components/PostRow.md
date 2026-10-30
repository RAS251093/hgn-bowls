# PostRow

`src/components/ui/PostRow.astro` is one news post in the admin list: a small cover, the title, status and when it last changed. The whole row is the link to edit it.

```astro
<ul aria-label="Posts" class="m-0 flex list-none flex-col gap-3 p-0">
  <PostRow title="Open day this Saturday" status="draft" meta="Last edited 14:32" editHref="/admin/edit?id=01J…" />
</ul>
```

**Props:** `title`, `status`, `meta` (the last-edited or published line), `editHref` and `thumbnail` (an image URL; decorative, `alt=""`, since the title names the post). A post without a cover shows the club photo the site uses for it.
- Rows are `bg-card` with a 1px border and `rounded-lg` corners, stacked with 12px gaps. Hover darkens the border to `input` and underlines the title.
- The title is the only link, named by the title alone, and stretched over the row, so a click anywhere opens the editor and a screen reader hears each post once. When it has keyboard focus the focus ring goes round the whole row (`focus-ring-within-link`).
- No buttons on the row. Deleting happens in the editor, where it acts at once and offers Undo, so a slip on a busy list can't remove a post.
- The page's one filled button is Write a post, above the list.
- `data-thumbnail`, `data-title`, `data-status` and `data-meta` let a page clone the row from a `<template>` and fill it in the browser, as text.
