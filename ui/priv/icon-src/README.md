# App icon source

`icon-src.svg` is the source for the favicon / Mac dock icon — a grayscale version
of `priv/static/images/logo.svg` on a dark rounded tile. It lives here (outside
`priv/static`) so it is **not** web-served; only the generated PNGs ship.

## Regenerate the icons (macOS)

```bash
cd priv
rm -rf /tmp/iconout && mkdir -p /tmp/iconout
qlmanage -t -s 512 -o /tmp/iconout icon-src/icon-src.svg
BASE=/tmp/iconout/icon-src.svg.png

cp "$BASE"                      static/images/icon-512.png
sips -z 192 192 "$BASE" --out   static/images/icon-192.png
sips -z 180 180 "$BASE" --out   static/apple-touch-icon.png
sips -z 32  32  "$BASE" --out   static/images/favicon-32.png
sips -z 16  16  "$BASE" --out   static/images/favicon-16.png
```

`favicon.svg` (transparent, tab icon) is hand-maintained in `static/favicon.svg`.
Head links + the web manifest live in `lib/council_hub_ui_web/components/layouts/root.html.heex`
and `static/site.webmanifest`.
