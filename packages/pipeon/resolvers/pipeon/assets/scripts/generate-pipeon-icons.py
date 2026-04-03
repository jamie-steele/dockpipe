#!/usr/bin/env python3
"""Generate Pipeon P-mark PNG (extension icon), ICO (browser tab), and matching SVG favicons."""
from __future__ import annotations

import base64
import os
import sys

from PIL import Image, ImageDraw, ImageFont

try:
    RESAMPLE_LANCZOS = Image.Resampling.LANCZOS
except AttributeError:
    RESAMPLE_LANCZOS = Image.LANCZOS

def _repo_root() -> str:
    d = os.path.dirname(os.path.abspath(__file__))
    while True:
        parent = os.path.dirname(d)
        if parent == d:
            break
        ext = os.path.join(
            d,
            "packages",
            "pipeon",
            "resolvers",
            "pipeon",
            "vscode-extension",
        )
        if os.path.isdir(ext):
            return d
        d = parent
    raise RuntimeError(
        "could not find dockpipe repo root (expected packages/pipeon/resolvers/pipeon/vscode-extension) from %s"
        % __file__
    )


REPO_ROOT = _repo_root()

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


def write_svgs(out_dir: str, png_path: str) -> None:
    """Write SVG wrappers that embed the canonical PNG icon."""
    with open(png_path, "rb") as f:
        png_b64 = base64.b64encode(f.read()).decode("ascii")
    svg = f"""<svg width="100%" height="100%" viewBox="0 0 256 256" xmlns="http://www.w3.org/2000/svg">
  <image width="256" height="256" href="data:image/png;base64,{png_b64}"/>
</svg>
"""
    os.makedirs(out_dir, exist_ok=True)
    for name in ("favicon.svg", "favicon-dark-support.svg"):
        path = os.path.join(out_dir, name)
        with open(path, "w", encoding="utf-8") as f:
            f.write(svg.strip() + "\n")
        print("wrote", path)


def main() -> int:
    # Single canonical tree for Pipeon branding + code-server favicons (shortcuts, Docker image, extension).
    ext_img = os.path.join(
        REPO_ROOT,
        "packages",
        "pipeon",
        "resolvers",
        "pipeon",
        "vscode-extension",
        "images",
    )
    os.makedirs(ext_img, exist_ok=True)

    png_path = os.path.join(ext_img, "icon.png")
    if os.path.exists(png_path):
        src = Image.open(png_path).convert("RGBA")
        print("using existing", png_path)
    else:
        src = render_p(256)
        src.save(png_path)
        print("wrote", png_path)

    sizes = [16, 32, 48]
    ico_imgs = [src.resize((s, s), RESAMPLE_LANCZOS) for s in sizes]
    ico_path = os.path.join(ext_img, "favicon.ico")
    ico_imgs[0].save(
        ico_path,
        format="ICO",
        sizes=[(s, s) for s in sizes],
        append_images=ico_imgs[1:],
    )
    print("wrote", ico_path)

    write_svgs(ext_img, png_path)
    return 0


if __name__ == "__main__":
    try:
        from PIL import Image as _  # noqa: F401
    except ImportError:
        print("Pillow required: pip install Pillow", file=sys.stderr)
        sys.exit(1)
    raise SystemExit(main())
