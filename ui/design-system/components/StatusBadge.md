# StatusBadge

`src/components/ui/StatusBadge.astro` marks a news post as Published or Draft in the admin area.

```astro
<StatusBadge status="draft" />
```

**Props:** `status`, `published` or `draft`.
- Always a word and an icon (check or pencil), never colour alone: `bg-affirmative` for Published, `bg-warning` for Draft.
- 4px corners (`rounded-sm`), not a pill.
- Admin only. The public site has no status colours.
