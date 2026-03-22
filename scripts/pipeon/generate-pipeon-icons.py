#!/usr/bin/env python3
"""Generate Pipeon P-mark PNG (extension icon), ICO (browser tab), and matching SVG favicons."""
from __future__ import annotations

import os
import sys

from PIL import Image, ImageDraw, ImageFont

REPO_ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", ".."))

# Pipeon mark: deep blue tile + light "P"
BG = (20, 50, 82, 255)
FG = (232, 240, 255, 255)


def _font(size: int) -> ImageFont.FreeTypeFont | ImageFont.ImageFont:
    for path in (
        "/usr/share/fonts/truetype/dejavu/DejaVuSans-Bold.ttf",
        "/usr/share/fonts/truetype/liberation/LiberationSans-Bold.ttf",
        "/usr/share/fonts/truetype/noto/NotoSans-Bold.ttf",
    ):
        try:
            return ImageFont.truetype(path, size)
        except OSError:
            continue
    return ImageFont.load_default()


def render_p(size: int) -> Image.Image:
    img = Image.new("RGBA", (size, size), BG)
    draw = ImageDraw.Draw(img)
    font = _font(max(10, int(size * 0.52)))
    text = "P"
    bbox = draw.textbbox((0, 0), text, font=font)
    tw = bbox[2] - bbox[0]
    th = bbox[3] - bbox[1]
    x = (size - tw) / 2 - bbox[0]
    y = (size - th) / 2 - bbox[1]
    draw.text((x, y), text, font=font, fill=FG)
    return img


def write_svgs(out_dir: str) -> None:
    """SVG favicons (viewBox matches upstream code-server 147 for drop-in)."""
    svg = """<svg width="100%" height="100%" viewBox="0 0 147 147" xmlns="http://www.w3.org/2000/svg">
  <rect width="147" height="147" rx="32" fill="#143252"/>
  <text x="50%" y="54%" dominant-baseline="middle" text-anchor="middle"
    font-family="DejaVu Sans, Liberation Sans, Arial, Helvetica, sans-serif"
    font-weight="700" font-size="78" fill="#e8f0ff">P</text>
</svg>
"""
    dark = """<svg width="100%" height="100%" viewBox="0 0 147 147" xmlns="http://www.w3.org/2000/svg">
  <style>
    @media (prefers-color-scheme: dark) {
      rect { fill: #1a5080; }
    }
  </style>
  <rect width="147" height="147" rx="32" fill="#143252"/>
  <text x="50%" y="54%" dominant-baseline="middle" text-anchor="middle"
    font-family="DejaVu Sans, Liberation Sans, Arial, Helvetica, sans-serif"
    font-weight="700" font-size="78" fill="#e8f0ff">P</text>
</svg>
"""
    os.makedirs(out_dir, exist_ok=True)
    for name, content in (("favicon.svg", svg), ("favicon-dark-support.svg", dark)):
        path = os.path.join(out_dir, name)
        with open(path, "w", encoding="utf-8") as f:
            f.write(content.strip() + "\n")
        print("wrote", path)


def main() -> int:
    ext_img = os.path.join(REPO_ROOT, "contrib/pipeon-vscode-extension", "images")
    cs_dir = os.path.join(REPO_ROOT, "templates", "core", "assets", "images", "code-server")
    os.makedirs(ext_img, exist_ok=True)
    os.makedirs(cs_dir, exist_ok=True)

    img128 = render_p(128)
    png_path = os.path.join(ext_img, "icon.png")
    img128.save(png_path)
    print("wrote", png_path)

    sizes = [16, 32, 48]
    ico_imgs = [render_p(s).convert("RGBA") for s in sizes]
    ico_path = os.path.join(cs_dir, "favicon.ico")
    ico_imgs[0].save(
        ico_path,
        format="ICO",
        sizes=[(s, s) for s in sizes],
        append_images=ico_imgs[1:],
    )
    print("wrote", ico_path)

    write_svgs(cs_dir)
    return 0


if __name__ == "__main__":
    try:
        from PIL import Image as _  # noqa: F401
    except ImportError:
        print("Pillow required: pip install Pillow", file=sys.stderr)
        sys.exit(1)
    raise SystemExit(main())
