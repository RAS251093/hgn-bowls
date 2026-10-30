# Hough Green Park Bowls Club design system

A modern, green-led look for a friendly crown green bowls club in Widnes. Club green carries the identity on a cool, green-tinted near-white; one highly legible sans in three weights; soft curved rectangles; large, clear text for members of every age. Built on a few firm principles: tokens only, flat surfaces, one highlight colour used sparingly, every state designed, nothing that looks generated.

The site has two audiences. Visitors and members read the public pages (Home, What's on, News, Join us, Find us), often on a phone and many of them older. Club admins, the least technical people involved, write news posts in a small admin area that must feel like writing an email.

## 1. Non-negotiables

1. **Tokens only.** Every colour, radius, text style and width comes from `tokens.json`, through the Tailwind utilities it generates (`bg-primary`, `text-heading-xl`, `rounded-lg`, `max-w-content`). Tailwind's default colours, sizes, radii and shadows are switched off, so an off-system class simply doesn't exist. No hex values and no arbitrary values (`p-[13px]`) in pages or components.
2. **Light only, cool neutrals, never pure black or white.** The page is `background`, a near-white tinted toward club green; raised surfaces are `card`. No cream, sand or beige, and no dark mode. Club green bands (header and hero as one block, the footer, the contact form) are the only inverted surfaces.
3. **One family, three weights, the fixed scale only.** Atkinson Hyperlegible Next 400 for reading, 500 for labels, 600 for headings. It was designed by the Braille Institute for low-vision readers, so letters that look alike elsewhere (I, l and 1; O and 0; b and d) stay distinct. Body is `body-md`, 18px. Nothing smaller than 16px. No sizes between the steps.
4. **4px spacing, curved rectangles capped at 12px.** Spacing comes from `space-1` to `space-16`. Corners are `radius-sm` (badges), `radius-md` (buttons, fields, tiles) and `radius-lg` (cards, photos, green panels), the cap. No pills; the roundel logo is the only circle. Nested corners reduce concentrically.
5. **Flat.** Surfaces separate by fill, a 1px `border` and spacing, never by shadow. Group with a filled surface (`secondary`, `card`), not a line. Lines appear only between rows of a list of like records (events, table rows).
6. **Green is the club; turf is the highlight.** `primary` carries identity. `accent` (turf, a fresh light green) appears only on green, for the one thing to notice: the main action in the hero, Join us, the current page, the roundel. One turf action per green band. Status colours exist only in the admin area.
7. **Motion is quiet and quick.** Colour and border changes over 120ms, the FAQ chevron over 200ms, the undo bar over 6 seconds. Nothing bounces, slides in on scroll or loops. Every animation stops under `prefers-reduced-motion`.
8. **Every interactive element has its states.** Hover, the one focus ring, a 1px press on buttons, `disabled` tokens (never opacity) and `cursor: pointer`. Icon-only controls carry an `aria-label` and a tooltip.
9. **One filled button per container.** Everything else steps down to outline. Labels never wrap. Delete acts immediately and offers Undo; there's no "Are you sure?".
10. **Copy is sentence case, second person, contracted, British.** No Title Case, no capitals as decoration, no ampersands, no em or en dashes, no emoji. Unknown facts stay as bracketed placeholders like [POSTCODE], never invented.
11. **Accessibility is measured.** WCAG 2.2 AA against the real ground: 4.5:1 for text, 3:1 for large text, field edges, icons and the focus ring. Every tap target is at least 44px. One `h1` per page, landmarks, alt text on every meaningful photo, state never by colour alone.
12. **Lucide icons, outlined, `currentColor`, 20px.** Used where an icon does real work (add, calendar, error, status), never as a tile on every card.

## 2. Content fundamentals

The voice is a helpful club member at the gate: warm, clear and unhurried. "You" for the reader, "we" for the club. Contractions throughout (you'll, it's, don't). British spelling (colour, centre, programme).
- Buttons are {verb} + {noun}: "Come and try it", "Get directions", "Add to my calendar", "Publish post", "Delete post". One label per intent, the same everywhere it appears.
- Nav and page names are fixed: Home, What's on, News, Find us. Join us joins them when the club has an online sign-up.
- Errors say what to do: "Enter an email address or a phone number so we can reply". Never "Invalid input".
- Empty states lead with the action: "Nothing on the calendar yet. See the calendar in Google".
- Dates are written out: "Saturday 18 April", "Sat 18 Apr" in tight spaces. Times use the 24-hour clock in the admin area and "2pm" on public pages.
- Spell out "and". One exclamation mark per page at most. FAQ questions are the only copy that ends in a question mark.
- Real content only: the club's own photos, events from its calendar, its own news. Missing facts (prices, postcode, phone) are bracketed placeholders until the committee confirms them.

## 3. Colour

| Token                                                                                 | Use                                                                              |
| ------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------- |
| `background`                                                                          | Cool green-tinted page ground                                                    |
| `card`                                                                                | Cards, fields, admin rows: a hair lighter than the page                          |
| `secondary`                                                                           | Grouping fill: alternate section band, callouts, outline-button hover            |
| `foreground` / `muted-foreground`                                                     | Body text / helper text, times, captions                                         |
| `meta-foreground`                                                                     | Fresh green for dates and meta lines on light grounds                            |
| `primary` / `primary-hover`                                                           | Club green: header, hero, footer, headings, links, the filled button / its hover |
| `primary-foreground` / `primary-muted-foreground`                                     | Text on green / secondary text on green                                          |
| `primary-border`                                                                      | Decorative hairline on green                                                     |
| `accent` / `accent-foreground`                                                        | Turf, on green only / green text on turf                                         |
| `border` / `input`                                                                    | Decorative edges / interactive field edges (3:1 or better)                       |
| `ring` / `ring-on-primary`                                                            | Focus ring on light grounds / on green                                           |
| `affirmative`, `warning`, `info`, `destructive`, `destructive-tint` (+ `-foreground`) | Admin status, notices and errors only, always with a word or icon                |
| `disabled` / `disabled-foreground`                                                    | Disabled fills / disabled text and edges                                         |

A page runs: header and hero as one green block, then `background` and `secondary` bands alternating, then the green footer. Most of the page is light; the green carries the weight at the top and bottom. Never put `accent` on a light ground.

## 4. Typography

| Style          | Use                                                                      |
| -------------- | ------------------------------------------------------------------------ |
| `heading-hero` | The page `h1` in the hero, once per page (36px on phones)                |
| `heading-xl`   | Section titles; admin page titles                                        |
| `heading-lg`   | Post subheads, the contact form title, the date tile numeral             |
| `heading-md`   | The workhorse: event, card, step, tier and FAQ titles; admin post titles |
| `heading-sm`   | The wordmark and small group titles                                      |
| `body-lg`      | Hero ledes and the Find us paragraph, capped at 40ch                     |
| `body-md`      | Default body text                                                        |
| `body-long`    | News post body and the editor, inside `measure`                          |
| `body-sm`      | Helper text, footer, captions, admin meta                                |
| `label`        | Buttons, nav links, field labels                                         |
| `meta`         | Dates and meta lines, in `meta-foreground`                               |

Headings are `primary` on light grounds and `primary-foreground` on green. Never change the tracking: the face is spaced for legibility. Prices and dates use lining tabular figures. Atkinson Hyperlegible Next is self-hosted from `public/fonts` (declared in `src/styles/tokens.css`) so pages make no third-party requests.

## 5. Layout and spacing

- Content sits in `container` (1180px) with a 24px (`space-6`) gutter each side, 16px on phones. Sections have 64px (`space-16`) vertical padding, 48px on phones.
- News posts and other long reading cap at `measure` (720px); standalone forms at `form-measure` (640px). Both centre.
- Two-up splits (hero, Find us) use flex-wrap with a 48px gap and stack below 768px. Card grids use `repeat(auto-fit, minmax(280px, 1fr))` with 32px gaps.
- Card padding is 24px; gaps between fields 16px; label to field 8px.
- Mobile first: everything works at 360px wide, nothing scrolls sideways, every target is at least `target-min`.
- The three club facts under the hero are plain heading-and-sentence columns, not a grid of icon cards.

## 6. Depth, borders and imagery

- No shadows anywhere. Cards and rows use a 1px `border`; fields and outline buttons use a 1.5px edge (`border-width`).
- A separator line is full width or absent. Use them between event rows and table rows only; elsewhere group with `secondary` or `card` fill.
- Photography is the club's own (in `src/assets`), with `radius-lg` corners. No frames, overlays, gradients, filters or stock images.
- The roundel is the only mark. If the club has an official crest, it replaces the roundel.

## 7. Interaction

| State    | Treatment                                                                                                                                             |
| -------- | ----------------------------------------------------------------------------------------------------------------------------------------------------- |
| Hover    | Filled buttons lighten to `primary-hover`; outline buttons gain a `secondary` wash; links thicken their underline; cards darken their edge to `input` |
| Focus    | One ring everywhere: 3px solid `ring`, offset 2px. Inside an `on-primary` band it becomes `ring-on-primary` (turf). Never removed                     |
| Press    | Buttons move down 1px                                                                                                                                 |
| Disabled | `disabled` fill, `disabled-foreground` text, `cursor: not-allowed`, no hover                                                                          |
| Error    | `destructive` label, edge and message with an icon, plus a page-level Callout on submit                                                               |
| Loading  | Placeholder blocks in `secondary` shaped like the content; no spinners on public pages                                                                |

Links in running text are always underlined. Destructive admin actions act at once and show an UndoToast for 6 seconds. Publishing shows "Published. It'll be on the website within a couple of minutes".

## 8. Iconography

Lucide, outlined, 2px stroke, `currentColor`, 20px in buttons and messages (24px for the FAQ chevron, 16px inside badges). The icons in use: plus, calendar-plus, circle-alert, check, pencil, chevron-down, arrow-left, map-pin, external-link, and the editor's bold, italic, heading, list, link and image. No emoji or unicode glyphs as icons (write "Back to all posts" with an arrow-left icon, not an arrow character). Decorative icons are `aria-hidden`; icon-only buttons (the editor toolbar) carry `aria-label` and a tooltip.

## 9. Components

Components are Astro components in `src/components/ui/`, styled with Tailwind utilities generated from the tokens. Each guide in `design-system/components/` gives the props and when to use it; `/design-system` renders all of them under `npm run dev` only; it's never built, so visitors can't reach it.
- Navigation: SiteHeader, Hero, SiteFooter
- Actions: Button (`primary`, `accent`, `outline`, `destructive`; default and `sm`)
- Content: EventList, NewsCard, StepList, MembershipCard, Faq, Prose
- Forms: Field
- Admin: PostRow, StatusBadge, Callout, UndoToast

Give any club green band the `on-primary` class. It sets the green fill and light text, switches the focus ring to turf, and turns on the `on-primary:` variant so components inside adapt (outline buttons, fields, error messages).

### Tailwind, in short

| Need                    | Use                                                                                                                                                                                                                                   |
| ----------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Colours                 | `bg-background`, `bg-card`, `bg-secondary`, `bg-primary`, `text-foreground`, `text-muted-foreground`, `text-meta-foreground`, `bg-accent`, `border-border`, `border-input` and the status pairs                                       |
| Type                    | `text-heading-hero`, `text-heading-xl`, `text-heading-lg`, `text-heading-md`, `text-heading-sm`, `text-body-lg`, `text-body-md`, `text-body-long`, `text-body-sm`, `text-label`, `text-meta` (each sets size, line height and weight) |
| Spacing                 | Tailwind's 4px steps: `gap-2` 8px, `p-4` 16px, `p-6` 24px, `gap-8` 32px, `gap-12` 48px, `py-16` 64px                                                                                                                                  |
| Corners                 | `rounded-sm` 4px, `rounded-md` 8px, `rounded-lg` 12px                                                                                                                                                                                 |
| Widths                  | `max-w-content` 1180px, `max-w-measure` 720px, `max-w-form` 640px                                                                                                                                                                     |
| Field and outline edges | `border-control` (1.5px)                                                                                                                                                                                                              |
| On a green band         | `on-primary:` variant                                                                                                                                                                                                                 |

## 10. Not this

These read as generated and are out: tracked capital eyebrows above headings, numbers as decoration, giant stat blocks, rows of identical icon cards, coloured side stripes on quotes or callouts, pill badges, shadows with borders, gradients, glass, cream or beige grounds, Title Case, em dashes, and placeholder people or lorem ipsum.

## 11. Before you ship

- [ ] No hex values or arbitrary values in the page; everything is a token utility or a component
- [ ] One `h1`, landmarks, alt text on meaningful photos, `alt=""` on decorative ones
- [ ] Text contrast checked on its real ground; turf never on a light ground
- [ ] Every link and button shows hover, the focus ring and a 44px target; keyboard reaches everything
- [ ] One filled button per container; no label wraps
- [ ] Loading, empty and error states exist for events, the contact form and the admin editor
- [ ] Copy is sentence case, British, contracted, with no ampersands, dashes or emoji
- [ ] Works at 360px and at full desktop width with nothing scrolling sideways
- [ ] Animations stop under reduced motion
- [ ] Lighthouse: performance 95 or more on mobile, accessibility 100

## Files

| Path                            | What                                                                                                       |
| ------------------------------- | ---------------------------------------------------------------------------------------------------------- |
| `design-system/tokens.json`     | Every token value: the source of truth                                                                     |
| `src/styles/tokens.css`         | Generated by `npm run tokens`: the tokens as CSS custom properties and Tailwind theme, plus the font faces |
| `src/styles/site.css`           | The site stylesheet: Tailwind, the tokens, the `on-primary` variant and base styles                        |
| `src/components/ui/`            | The components                                                                                             |
| `design-system/components/*.md` | Usage guide for each component                                                                             |
| `src/dev/design-system.astro`   | Every component on one page, for checking changes (`npm run dev` only)                                     |
| `public/logos/`                 | The roundel, light (on green) and green (on light grounds)                                                 |
| `public/fonts/`                 | Atkinson Hyperlegible Next 400, 500 and 600 (SIL Open Font License)                                        |

To change a token, edit `tokens.json` and run `npm run tokens`. Never edit `tokens.css` by hand.
