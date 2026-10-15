# PostRow

`src/components/ui/PostRow.astro` is one news post in the admin list: thumbnail, title, status, date and actions.

```astro
<ul class="m-0 flex list-none flex-col gap-3 p-0">
  <PostRow title="Open day this Saturday" status="draft" meta="Last edited 14:32" thumbnail={cover} editHref="/admin/posts/open-day" />
</ul>
```

**Props:** `title`, `status`, `meta` (the last-edited or published line), `thumbnail` (`ImageMetadata`, decorative) and `editHref`.

- Rows are `bg-card` with a 1px border and `rounded-lg` corners, stacked with 12px gaps.
- Edit post is an `outline` `sm` Button; Delete post is `destructive` `sm` and carries `data-delete-post` for the script to hook. The page's one filled button is Write a post, above the list.
- Delete removes the row at once and shows an UndoToast. There's no "Are you sure?" dialog.
