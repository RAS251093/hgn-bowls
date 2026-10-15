# Prose

`src/components/ui/Prose.astro` styles the body of a news post: paragraphs, subheadings, lists, links and pull quotes.

```astro
<Prose><Content /></Prose>
```

The default slot is the post HTML (Markdown or the editor's output). It caps the line length at `max-w-measure` (720px).

- Body is `text-body-long` (19px on 32px). Subheadings are `text-heading-lg` in club green.
- Links are underlined in `text-primary` and thicken on hover.
- A pull quote is a `blockquote` on `bg-secondary` at 20px medium weight, with the member's name in a `footer`. No side stripe.
- The admin editor preview uses the same component so what admins see is what members get.
