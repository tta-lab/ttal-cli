---
name: image-tools
description: CLI image manipulation — convert PNG/JPG to SVG, remove watermarks, resize, crop, and edit raster images using ImageMagick and vtracer
---

# Image Tools

CLI-first image manipulation using ImageMagick (raster) and vtracer (raster-to-vector).

## Prerequisites

```bash
# Check if installed
which magick && echo "imagemagick OK" || echo "need: brew install imagemagick"
which vtracer && echo "vtracer OK" || echo "need: cargo install vtracer"
```

Install if missing:
```bash
brew install imagemagick    # raster image editing
cargo install vtracer       # raster-to-vector conversion (Rust)
```

## Raster-to-SVG Conversion

### Simple flat icon (single/few colors)
Best for logos, icons, flat illustrations:

```bash
vtracer --input input.png --output output.svg \
  --colormode color \
  --filter_speckle 10 \
  --color_precision 4 \
  --corner_threshold 60 \
  --segment_length 6 \
  --gradient_step 32
```

### High-detail image
For complex images, increase precision:

```bash
vtracer --input input.png --output output.svg \
  --colormode color \
  --filter_speckle 4 \
  --color_precision 6 \
  --corner_threshold 60 \
  --segment_length 4
```

### Black-and-white / monochrome
For line art, text, or single-color designs:

```bash
vtracer --input input.png --output output.svg --colormode bw
```

### Tuning guide
- **file too large** → increase `filter_speckle`, `gradient_step`, decrease `color_precision`
- **too rough/blocky** → decrease `segment_length`, `filter_speckle`
- **too many colors** → decrease `color_precision` (1-8, lower = fewer colors)

## Raster Editing with ImageMagick

### Remove watermark / overlay
Paint over a region with a solid color (e.g. matching background):

```bash
# Get image dimensions first
magick identify input.png

# Paint rectangle over bottom-right corner (100x100 area)
magick input.png -fill "#0F0F0F" -draw "rectangle 1180,1180 1280,1280" output.png
```

### Resize
```bash
# Resize to specific width, maintain aspect ratio
magick input.png -resize 512x output.png

# Resize to exact dimensions
magick input.png -resize 512x512! output.png

# Resize to fit within bounds (no upscale)
magick input.png -resize 512x512\> output.png
```

### Crop
```bash
# Crop to 500x500 from top-left
magick input.png -crop 500x500+0+0 output.png

# Crop from center
magick input.png -gravity center -crop 500x500+0+0 output.png
```

### Format conversion
```bash
magick input.png output.jpg
magick input.jpg output.webp
magick input.png -quality 85 output.jpg
```

### Trim whitespace / transparent borders
```bash
magick input.png -trim +repage output.png
```

### Threshold (prepare for BW tracing)
```bash
magick input.png -threshold 50% output.pbm
```

## Common Workflows

### Clean image then convert to SVG
When source image has watermarks, logos, or artifacts:

```bash
# 1. Check dimensions
magick identify input.jpg

# 2. Remove unwanted elements (paint over with background color)
magick input.jpg -fill "#BACKGROUND" -draw "rectangle x1,y1 x2,y2" clean.jpg

# 3. Convert to SVG
vtracer --input clean.jpg --output output.svg --colormode color \
  --filter_speckle 10 --color_precision 4 --corner_threshold 60 \
  --segment_length 6 --gradient_step 32

# 4. Check output size (aim for <50KB for icons)
ls -la output.svg
```

### Create favicon from logo
```bash
# Resize to favicon sizes
magick logo.png -resize 32x32 favicon-32.png
magick logo.png -resize 16x16 favicon-16.png

# Or create .ico with multiple sizes
magick logo.png -resize 256x256 -define icon:auto-resize=256,128,64,48,32,16 favicon.ico
```

## Decision Guide

| Task | Tool |
|------|------|
| PNG/JPG → SVG | vtracer |
| Resize, crop, format convert | magick |
| Remove watermark | magick (paint over) → then vtracer if SVG needed |
| Trim borders | magick |
| Create favicon | magick |
