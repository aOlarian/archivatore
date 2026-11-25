# MAP
# §1 Pure-stdlib PNG & ICO icon generator for Archivatore  L5-L85

# §1 Pure-stdlib PNG & ICO icon generator for Archivatore
import os
import struct
import zlib

OUT_DIR = os.path.dirname(os.path.abspath(__file__))


def _png_chunk(tag: bytes, data: bytes) -> bytes:
    crc = zlib.crc32(tag + data) & 0xFFFFFFFF
    return struct.pack(">I", len(data)) + tag + data + struct.pack(">I", crc)


def encode_png(rgba: bytearray, size: int) -> bytes:
    stride = size * 4
    raw = bytearray()
    for y in range(size):
        raw.append(0)
        raw.extend(rgba[y * stride : (y + 1) * stride])
    ihdr = struct.pack(">IIBBBBB", size, size, 8, 6, 0, 0, 0)
    idat = zlib.compress(bytes(raw), 9)
    return b"\x89PNG\r\n\x1a\n" + _png_chunk(b"IHDR", ihdr) + _png_chunk(b"IDAT", idat) + _png_chunk(b"IEND", b"")


def blend(buf: bytearray, size: int, x: int, y: int, r: int, g: int, b: int, a: int) -> None:
    if x < 0 or y < 0 or x >= size or y >= size or a <= 0:
        return
    idx = (y * size + x) * 4
    if a >= 255:
        buf[idx], buf[idx + 1], buf[idx + 2], buf[idx + 3] = r, g, b, 255
        return
    alpha = a / 255.0
    inv = 1.0 - alpha
    buf[idx] = int(r * alpha + buf[idx] * inv)
    buf[idx + 1] = int(g * alpha + buf[idx + 1] * inv)
    buf[idx + 2] = int(b * alpha + buf[idx + 2] * inv)
    buf[idx + 3] = min(255, int(a + buf[idx + 3] * inv))


def render_icon(size: int) -> bytearray:
    buf = bytearray(size * size * 4)
    cx, cy = (size - 1) / 2.0, (size - 1) / 2.0
    pad = size * 0.06
    rad = size * 0.22
    half = size / 2.0 - pad

    for y in range(size):
        for x in range(size):
            dx = max(abs(x - cx) - (half - rad), 0.0)
            dy = max(abs(y - cy) - (half - rad), 0.0)
            dist = (dx * dx + dy * dy) ** 0.5 - rad
            if dist <= 0.7:
                cov = min(1.0, max(0.0, 0.7 - dist))
                t = y / max(1, size - 1)
                r = int(20 + 12 * (1 - t))
                g = int(34 + 18 * (1 - t))
                b = int(38 + 20 * (1 - t))
                blend(buf, size, x, y, r, g, b, int(255 * cov))

    # Outer capsule frame (turquoise #3CC4C8) + inner zipper/core bars
    for y in range(size):
        ny = (y - cy) / (size * 0.34)
        for x in range(size):
            nx = (x - cx) / (size * 0.34)
            # Isometric/geometric capsule box
            if -0.88 <= nx <= 0.88 and -0.82 <= ny <= 0.82:
                border = abs(nx) > 0.68 or abs(ny) > 0.62
                if border:
                    blend(buf, size, x, y, 60, 196, 200, 255)
                elif -0.14 <= nx <= 0.14:
                    # Center zipper stripe alternating teeth
                    tooth = int((ny + 0.7) * 6) % 2 == 0
                    if tooth and nx <= 0.02 or (not tooth and nx >= -0.02):
                        blend(buf, size, x, y, 240, 237, 229, 245)
                elif -0.52 <= nx <= -0.24 and -0.42 <= ny <= 0.42:
                    blend(buf, size, x, y, 13, 116, 120, 255)
                elif 0.24 <= nx <= 0.52 and -0.42 <= ny <= 0.42:
                    blend(buf, size, x, y, 13, 116, 120, 255)
    return buf


def main() -> None:
    png256 = encode_png(render_icon(256), 256)
    with open(os.path.join(OUT_DIR, "icon.png"), "wb") as f:
        f.write(png256)

    sizes = [16, 32, 48, 64, 256]
    images = [encode_png(render_icon(s), s) for s in sizes]
    header = struct.pack("<HHH", 0, 1, len(images))
    offset = 6 + 16 * len(images)
    entries = bytearray()
    for s, data in zip(sizes, images):
        w = 0 if s == 256 else s
        entries.extend(struct.pack("<BBBBHHII", w, w, 0, 0, 1, 32, len(data), offset))
        offset += len(data)
    with open(os.path.join(OUT_DIR, "icon.ico"), "wb") as f:
        f.write(header + bytes(entries) + b"".join(images))
    print("Generated icon.png and icon.ico")


if __name__ == "__main__":
    main()
