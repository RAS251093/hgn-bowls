# EventList

`src/components/ui/EventList.astro` shows upcoming fixtures and socials from the club's Google Calendar as rows with a green date tile.

```astro
<EventList events={[{ start: new Date("2027-04-17T13:00:00Z"), title: "Open bowling day", when: "2pm · Main green" }]} />
```

**Props:** `events`, each `{ start: Date, title, when, description? }`. Dates are formatted in en-GB, Europe/London.
- The date tile reads Sat / 17 / Apr, the weekday in `text-accent`. It is `aria-hidden`; the title starts with the full date for screen readers.
- Rows are separated by full-width `border-border` rules: a list of like records is the one place lines are right.
- An empty list shows "Nothing on the calendar yet" with a link to the Google calendar, never a blank space.
- Home shows the next 4 events; What's on groups all upcoming events under month headings in `text-heading-xl`.
- Loading: 4 rows of `bg-secondary` placeholder blocks, no spinner. Error: the same Google calendar link in place of the list.
- Pair it with an `outline` Add to my calendar Button, which subscribes members' phones to the club calendar.
