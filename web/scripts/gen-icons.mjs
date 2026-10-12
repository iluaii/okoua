// SPDX-License-Identifier: AGPL-3.0-or-later

// Generates the PWA icons in static/icons/ (PNG, no image tools needed):
// a red circle with a white dot on a dark background, matching favicon.svg.

import { mkdir, writeFile } from 'node:fs/promises';
import { deflateSync } from 'node:zlib';

const OUT = new URL('../static/icons/', import.meta.url);
const BG = [17, 20, 24];
const RED = [217, 58, 58];
const WHITE = [255, 255, 255];

/** Renders an icon: `scale` is the red circle's diameter relative to the size. */
function render(size, scale, rounded) {
	const px = new Uint8Array(size * size * 4);
	const c = size / 2;
	const rOuter = (size * scale) / 2;
	const rInner = rOuter * 0.4;
	const corner = rounded ? size * 0.22 : 0;
	const SS = 4; // supersampling for smooth edges
	for (let y = 0; y < size; y++) {
		for (let x = 0; x < size; x++) {
			let acc = [0, 0, 0, 0];
			for (let sy = 0; sy < SS; sy++) {
				for (let sx = 0; sx < SS; sx++) {
					const fx = x + (sx + 0.5) / SS;
					const fy = y + (sy + 0.5) / SS;
					if (!insideRoundedSquare(fx, fy, size, corner)) continue;
					const d = Math.hypot(fx - c, fy - c);
					const col = d <= rInner ? WHITE : d <= rOuter ? RED : BG;
					acc = [acc[0] + col[0], acc[1] + col[1], acc[2] + col[2], acc[3] + 255];
				}
			}
			const n = SS * SS;
			const i = (y * size + x) * 4;
			const a = acc[3] / n;
			// Colours were summed only over covered samples; un-premultiply.
			const covered = acc[3] / 255 || 1;
			px[i] = acc[0] / covered;
			px[i + 1] = acc[1] / covered;
			px[i + 2] = acc[2] / covered;
			px[i + 3] = a;
		}
	}
	return png(size, px);
}

function insideRoundedSquare(x, y, size, r) {
	if (r === 0) return true;
	const cx = Math.min(Math.max(x, r), size - r);
	const cy = Math.min(Math.max(y, r), size - r);
	return Math.hypot(x - cx, y - cy) <= r;
}

function png(size, rgba) {
	const raw = Buffer.alloc((size * 4 + 1) * size);
	for (let y = 0; y < size; y++) {
		raw[y * (size * 4 + 1)] = 0; // filter: none
		Buffer.from(rgba.buffer, y * size * 4, size * 4).copy(raw, y * (size * 4 + 1) + 1);
	}
	const ihdr = Buffer.alloc(13);
	ihdr.writeUInt32BE(size, 0);
	ihdr.writeUInt32BE(size, 4);
	ihdr.set([8, 6, 0, 0, 0], 8); // 8-bit RGBA
	return Buffer.concat([
		Buffer.from([0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a]),
		chunk('IHDR', ihdr),
		chunk('IDAT', deflateSync(raw, { level: 9 })),
		chunk('IEND', Buffer.alloc(0))
	]);
}

function chunk(type, data) {
	const len = Buffer.alloc(4);
	len.writeUInt32BE(data.length);
	const body = Buffer.concat([Buffer.from(type, 'ascii'), data]);
	const crc = Buffer.alloc(4);
	crc.writeUInt32BE(crc32(body));
	return Buffer.concat([len, body, crc]);
}

const CRC = Array.from({ length: 256 }, (_, n) => {
	let c = n;
	for (let k = 0; k < 8; k++) c = c & 1 ? 0xedb88320 ^ (c >>> 1) : c >>> 1;
	return c >>> 0;
});
function crc32(buf) {
	let c = 0xffffffff;
	for (const b of buf) c = CRC[(c ^ b) & 0xff] ^ (c >>> 8);
	return (c ^ 0xffffffff) >>> 0;
}

await mkdir(OUT, { recursive: true });
const icons = {
	'icon-192.png': render(192, 0.62, true),
	'icon-512.png': render(512, 0.62, true),
	// Maskable icons are cropped to a circle or squircle: keep the mark in the central safe zone.
	'maskable-512.png': render(512, 0.5, false),
	'apple-touch-icon.png': render(180, 0.6, false)
};
for (const [name, data] of Object.entries(icons)) {
	await writeFile(new URL(name, OUT), data);
	console.log(name, data.length, 'bytes');
}
