# Responsive UI guide for Timelapse

This notes the patterns we just applied so we can reuse them across pages.

## Cards and grids (cameras)
- Grid: `display: grid; grid-template-columns: repeat(auto-fit, minmax(240px, 1fr)); gap:16px;` — scales from phones to 4+ columns on desktop; handles 50–60 cards with vertical scroll.
- Card: single-column grid with padding and shadow; keep min-height modest (~260px) for density.
- Thumbnails: wrap in a `.thumb` with max-height (260px desktop / 220px mobile), `object-fit: contain`, and `background` fallback. Use `loading="lazy"` on `<img>`.
- Text blocks: keep `min-width:0` and avoid fixed widths so wrapping is clean; use pill badges for status.

## Tables (worker status) responsive pattern
- Wrap tables in `.table-wrap` (overflow-x: auto) for desktop resize protection.
- Add a mobile fallback stack: under 760px hide the table and render small cards instead. Structure:
  - Outer wrapper `.worker-table`
  - Inside: `<div class="table-wrap"><table>…</table></div>` and `<div class="stack">` with card rows for mobile.
- Keep table min-width (~720px) so columns don’t squish; rely on horizontal scroll on medium widths and stack on narrow.

## Topbars and buttons
- Use a `topbar` with flex, wrap, and a spacer; buttons as `.btn` + `.btn.primary`/`.ghost`.
- Ensure `flex-wrap: wrap` on topbars so controls don’t overflow when width shrinks.

## Typography/colors
- Shared tokens in `templates/_style.html` (bg, panel, accent, radii, shadow). Reuse this include on every page.

## Breakpoints we used
- =700px: tighter card padding, thumb max-height 220px, grid gap 12px.
- =760px: worker table hides; stacked cards show.

## How to apply to other pages
- Include `_style.html`.
- Place primary content in a `.page` container.
- For any new card lists, use the grid pattern above and `object-fit: contain` on media.
- For any tables, wrap with `.table-wrap`; if important on mobile, add a `.stack` fallback with key fields.

## Reusable snippets
- Grid: `class="cards"` on the container; each card structure: thumb + meta + actions.
- Worker/mobile card fields: label (small, uppercase) + value; include a status dot.

## Performance
- Use `loading="lazy"` on all snapshots; keep max-height caps to avoid enormous portrait images pushing content down.

## Next
- Mirror these patterns to other template pages (renders list, jobs, add/edit forms) if more tables or lists are added.
