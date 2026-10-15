# StepList

`src/components/ui/StepList.astro` is the numbered how-to-join sequence on the Join us page.

```astro
<StepList steps={[{ title: "Come to an open day", body: "Check What's on for the next open bowling day." }]} />
```

**Props:** `steps`, 2 to 4 items of `{ title, body }`. `body` may contain a link.

- Numbers are used only because the order matters. Don't number anything that isn't a real sequence.
- The number sits in a 44px club green square with `rounded-md` corners and a turf numeral. The roundel logo is the only circle in the system.
- Steps are `bg-card` with a 1px border, `rounded-lg` corners and 24px padding.
