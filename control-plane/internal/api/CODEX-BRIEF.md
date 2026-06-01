# Task: premium landing page for Watchclaw

Rewrite `landing.html` (in this directory) into a genuinely premium product landing page for **Watchclaw**, a sovereign self-hosted RMM (remote monitoring & management) for Europe. The current landing.html is a weak attempt — replace it entirely with something Apple / Antigravity-grade.

## Reference — match this aesthetic exactly
Read the real source of antigravity.google that I saved:
- `../../../design-ref/antigravity-real.html`
- `../../../design-ref/antigravity-real.css`

Match their palette (refined dark greys ~#121317 / #18191D, NOT pure black), their Google Sans Flex variable font, large expressive tight-tracked headlines, generous whitespace, minimal restraint, subtle motion.

## Requirements
- Single self-contained HTML file (inline CSS + JS, no build step, no npm). External allowed ONLY: Google Fonts (Google Sans Flex + Google Sans Code).
- Signature animation like antigravity: a hero `<canvas>` with floating network nodes + links in zero-gravity that react to the mouse (repel). It is thematic — Watchclaw discovers networks. Elegant and subtle, not busy.
- Brand accent green `#00C46A`; secondary blue `#3279F9`. Dark grey background per their palette.
- Sections: fixed nav (logo + links + "Open dashboard" button), hero (canvas + big headline + sub + two CTAs), features (auto-discovery via ARP/mDNS/SSDP/SNMP; AI-native copilot; EU-sovereign with NIS2 and no US CLOUD Act; self-hosted one-binary), a "why sovereign" section with 4 stats (57k IEEE vendor IDs, 4 discovery layers, under 35MB agent, 100% self-hosted), final CTA, footer.
- Routes: this page is served at `/`, the dashboard link goes to `/app`.
- Accessibility: focus-visible outlines, prefers-reduced-motion (disable canvas + transitions), `<main>` landmark + skip link, WCAG AA contrast, aria-hidden on decorative SVG/canvas.
- Make it look expensive and intentional. Premium typography hierarchy is the single most important thing.

## Ownership
Overwrite exactly this file: `landing.html` (current directory). Write the full final HTML. Do not touch any other file.

## Definition of done
Opens standalone in a browser and looks like a top-tier SaaS landing; self-contained; accessible.
