# Callout

`src/components/ui/Callout.astro` is a filled notice that groups a short message with at most one link.

```astro
<Callout variant="info">
  <strong>Adding an event?</strong> Events come straight from the club's Google Calendar, so add them there as usual.
  <a slot="action" href="https://calendar.google.com">Open Google Calendar</a>
</Callout>
```

**Props:** `variant`: `default` (`bg-secondary`, for tips), `info` (`bg-info`, neutral admin notices) or `error` (`bg-destructive-tint`, the page-level error, announced with `role="alert"`). The default slot is the message; the `action` slot takes one link.

- One or two sentences. Pair an error Callout with the field-level messages on submit.
- No coloured side stripe and no border: the fill does the grouping.
