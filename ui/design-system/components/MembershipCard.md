# MembershipCard

`src/components/ui/MembershipCard.astro` shows one membership price on the Join us page.

```astro
<div class="grid gap-6 md:grid-cols-3">
  <MembershipCard name="Adult" price="[£00]" note="[Per year. What's included]" />
</div>
```

**Props:** `name`, `price` and `note` (one line on what's included).
- Cards sit on a `bg-secondary` band; all tiers share the same anatomy and none is highlighted as "best value".
- The price is `text-heading-xl` with lining tabular figures.
- Prices the committee hasn't confirmed stay as bracketed placeholders; never invent them.
