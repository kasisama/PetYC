# Design System: QQPET Admin · MiSide (米塔)

## 1. Visual Theme & Atmosphere
A cozy pink apartment operations console inspired by MiSide (米塔). Warm cream daylight, soft rose primary, refined anime-adjacent ornament (small hearts, bows) that never blocks data. Premium adult apartment feel, not kids stickers, not cyberpunk, not generic blue SaaS admin. Medium data density for long ops sessions.

## 2. Color Palette & Roles
- **Cream Canvas** (#FFF8F5) — Primary page background
- **Rose Mist** (#FCEEF2) — Secondary background wash
- **Pure Surface** (#FFFFFF) — Cards and panels
- **Whisper Pink Border** (#F0DDE4) — 1px card borders
- **Mita Rose** (#E85A8C) — Primary CTA, active nav, links
- **Soft Blush** (#F47BA8) — Hover / secondary primary
- **Charcoal Rose Ink** (#3A2A32) — Primary text
- **Muted Mauve** (#7A6570) — Secondary text
- **Lavender Dust** (#C9B4D0) — Info accents
- **Mint Success** (#7EC8C4) — Success states (not neon green)
- **Cream Amber** (#E8A54B) — Warning / not hot-reloaded
- **Deep Danger** (#C23B5A) — Delete, reset, dying states
- **Night Plum** (#1A1218) — Night theme background
- **Night Card** (#2C2028) — Night surfaces
- **Night Rose** (#FF7AAD) — Night primary
- **Uncanny Void** (#140F18) — "别的米塔" background
- **Cold Rose** (#D63A6A) — Uncanny primary

## 3. Typography
- **Display/Headline:** Plus Jakarta Sans or Manrope — slightly rounded, tracking relaxed for titles
- **Body:** Noto Sans / Source Sans 3 style clarity for Chinese UI
- **Mono numbers:** JetBrains Mono for currency, growth, affection columns
- **Banned:** Decorative script as body; horror pixel fonts; Inter-as-only-identity is OK to avoid if Plus Jakarta available

## 4. Components
- **Cards:** 12–16px radius, soft warm pink-tinted shadow, 1px whisper border
- **Buttons:** 10–12px radius; primary filled Mita Rose; danger deep rose; ghost outline soft
- **Inputs:** 8–10px radius, label above, error below in danger color
- **Sidebar:** Hallway wall metaphor; active item soft pink bar + tiny heart mark
- **Topbar:** Logo QQPET Admin, theme switch, 热重载, account menu
- **Tables:** Subtle pink zebra, hover blush row, compact ops action buttons
- **Modals:** Rounded apartment "window" cards; warm dimmed overlay (not pure black)
- **Danger zones:** Cracked thin border, restrained uncanny tone — no gore

## 5. Layout
- Desktop-first 1440px admin shell: left sidebar + top bar + content
- Chinese UI copy only
- Config pages use sub-nav for 9 schemas
- Distinguish 保存 vs 热重载 states visually

## 6. Anti-Patterns
- No generic blue/purple SaaS admin
- No cyber neon terminal
- No kids sticker overload
- No gore / jump-scare full-screen glitch
- No inventing fake metrics (revenue trends, online users)
- Decorations must never obscure form fields or table data
