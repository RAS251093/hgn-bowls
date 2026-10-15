# Button

`src/components/ui/Button.astro` is the one control for every action. It renders an `<a>` when you pass `href`, otherwise a `<button>`.

```astro
<Button href="/join">Come and try it</Button>
<Button variant="outline"><CalendarPlus aria-hidden="true" />Add to my calendar</Button>
<Button variant="destructive" size="sm">Delete post</Button>
```

**Props:** `variant` (`primary`, `accent`, `outline`, `destructive`; default `primary`), `size` (`default` 48px, `sm` 44px), `href`, plus any `<a>` or `<button>` attribute. Children are the label and an optional 20px Lucide icon.

- **One filled button per container.** On light grounds that's `primary` (club green); everything else in the cluster is `outline`. Inside a club green band (`on-primary`) the filled one is `accent` (turf), and `outline` turns light with a turf edge by itself.
- `accent` only ever sits on club green; on a light ground turf has too little contrast to mean anything.
- `destructive` is an outline in `destructive`, never a red fill. Delete acts at once and shows an UndoToast.
- Labels never wrap. If buttons don't fit, the row wraps, never the label.
- Hover, press (1px down), the focus ring and the `disabled` colours are built in. Never dim with opacity.

| Do | Don't |
| --- | --- |
| Come and try it, Get directions, Publish post | Click here, Submit, Delete (alone) |
| One filled button per card, form or band | Two filled buttons side by side |
