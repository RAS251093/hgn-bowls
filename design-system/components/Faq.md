# Faq

`src/components/ui/Faq.astro` is a list of common questions as native `<details>` disclosures.

```astro
<Faq items={[{ question: "What should I wear?", answer: "Flat-soled shoes, so the green isn't damaged." }]} />
```

**Props:** `items`, each `{ question, answer }`.

- Questions keep their question mark; this is the one place copy ends in one.
- Each item is `bg-card` with a 1px border and `rounded-lg` corners; the summary is a 44px+ target in `text-heading-md`.
- The chevron rotates on open over 200ms; with reduced motion it just flips.
- Keep answers to two or three sentences. Longer answers belong in a news post.
