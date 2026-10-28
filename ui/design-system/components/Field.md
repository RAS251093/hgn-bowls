# Field

`src/components/ui/Field.astro` is a label, optional help text, an input or textarea, and an error message, stacked.

```astro
<Field id="contact" label="Email or phone number" required help="We'll only use this to reply." />
<Field id="message" label="Message (optional)" multiline />
<Field id="contact" label="Email or phone number" required error="Enter an email address or a phone number so we can reply" />
```

**Props:** `id` and `label` (required), `help`, `error`, `multiline`, `rows`, plus any `<input>` attribute (`type`, `autocomplete`, `value`…). Help and error are wired to the control with `aria-describedby`; an error sets `aria-invalid`.
- Labels sit above the field in `text-label`, 1 to 3 words, sentence case. Never use a placeholder as the label.
- Only required fields are marked, with an asterisk.
- Field edges are 1.5px `border-input` (3:1 or better on every ground) with `rounded-md` corners; hover and focus turn them `border-primary`.
- Errors say what to do, never "Invalid input". The label, edge and message turn `destructive`, and the message carries an icon so colour isn't the only cue.
- On club green (the contact form, a `form` with the `on-primary` class) fields keep their light fill, the focus ring turns turf, and an error message sits on a `bg-destructive-tint` strip so it stays readable.
- Fields fill their column; standalone forms are capped at `max-w-form` (640px).
