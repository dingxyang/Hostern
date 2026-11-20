"""Draw Hostern's macOS icon with Pillow. Run from any directory."""

import argparse
from pathlib import Path

from PIL import Image, ImageDraw

SIZE = 1024
SCALE = 4
ROOT = Path(__file__).resolve().parents[1]


def scaled(values):
    return tuple(value * SCALE for value in values)


def draw_icon(background, foreground):
    side = SIZE * SCALE
    icon = Image.new("RGBA", (side, side), background)

    mask = Image.new("L", icon.size)
    ImageDraw.Draw(mask).rounded_rectangle(scaled((100, 100, 924, 924)), 185 * SCALE, fill=255)
    icon.putalpha(mask)

    draw = ImageDraw.Draw(icon)
    # Swept wings form the chevron, with a pointed bill and a forked tail.
    draw.polygon(
        [scaled(point) for point in (
            (261, 318), (461, 410), (517, 466),
            (550, 466), (588, 499), (550, 532), (517, 532),
            (461, 588), (261, 680), (322, 569), (423, 521),
            (361, 536), (384, 499), (361, 462), (423, 477),
            (322, 429),
        )],
        fill=foreground,
    )
    draw.polygon(
        [scaled(point) for point in ((610, 608), (770, 608), (736, 672), (576, 672))],
        fill=foreground,
    )
    return icon.resize((SIZE, SIZE), Image.Resampling.LANCZOS)


def make_preview(icon, path):
    preview = Image.new("RGB", (416, 320))
    draw = ImageDraw.Draw(preview)
    for row, (background, text) in enumerate((("#E9EDF5", "#242F43"), ("#242A37", "#EEF2FA"))):
        y = row * 160
        draw.rectangle((0, y, 415, y + 159), fill=background)
        for size, center in zip((128, 64, 32, 16), (84, 203, 292, 367)):
            sample = icon.resize((size, size), Image.Resampling.LANCZOS)
            preview.paste(sample, (center - size // 2, y + 68 - size // 2), sample)
            draw.text((center, y + 137), str(size) + " px", fill=text, anchor="mt")
    preview.save(path)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--out", type=Path, default=ROOT / "resources" / "icon.png")
    parser.add_argument("--preview", type=Path)
    parser.add_argument("--black", action="store_true", help="Draw the black-background variant.")
    args = parser.parse_args()
    colors = ("#000000", "#FFFFFF") if args.black else ("#FFFFFF", "#000000")
    icon = draw_icon(*colors)
    args.out.parent.mkdir(parents=True, exist_ok=True)
    icon.save(args.out)
    if args.preview:
        make_preview(icon, args.preview)


if __name__ == "__main__":
    main()
