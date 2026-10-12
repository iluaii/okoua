// SPDX-License-Identifier: AGPL-3.0-or-later

// Service worker: offline app shell, last-known data when offline, and push notifications.

import { self } from '$app/service-worker';
import { version } from '$app/env';
import { assets, immutable, prerendered } from '$app/manifest';

const SHELL = `shell-${version}`;
const DATA = 'data'; // last API responses, kept across versions

// Manifest paths are relative to the base path (no leading slash).
const abs = (p: string) => '/' + p.replace(/^\//, '');
const shellFiles = [...immutable, ...assets, ...prerendered].map((f) => abs(f.path));

self.addEventListener('install', (event) => {
	event.waitUntil(
		caches
			.open(SHELL)
			.then((c) => c.addAll(shellFiles))
			.then(() => self.skipWaiting())
	);
});

self.addEventListener('activate', (event) => {
	event.waitUntil(
		caches
			.keys()
			.then((keys) => Promise.all(keys.filter((k) => k !== SHELL && k !== DATA).map((k) => caches.delete(k))))
			.then(() => self.clients.claim())
	);
});

self.addEventListener('fetch', (event) => {
	const req = event.request;
	if (req.method !== 'GET') return;
	const url = new URL(req.url);
	if (url.origin !== self.location.origin) return;

	if (url.pathname.startsWith('/api/')) {
		// Snapshots that are useful offline: network first, last response as a fallback.
		if (url.pathname === '/api/alerts' || url.pathname.startsWith('/api/power/')) {
			event.respondWith(networkFirst(req, DATA));
		}
		return; // the SSE stream and everything else go straight to the network
	}

	if (req.mode === 'navigate') {
		event.respondWith(
			fetch(req).catch(async () => (await caches.match(url.pathname, { ignoreSearch: true })) ?? (await caches.match('/')) ?? Response.error())
		);
		return;
	}

	if (shellFiles.includes(url.pathname)) {
		event.respondWith(caches.match(req).then((hit) => hit ?? fetch(req)));
	}
});

async function networkFirst(req: Request, cacheName: string): Promise<Response> {
	const cache = await caches.open(cacheName);
	try {
		const res = await fetch(req);
		if (res.ok) await cache.put(req, res.clone());
		return res;
	} catch {
		const hit = await cache.match(req);
		if (!hit) throw new Error('offline and not cached');
		// Tell the page this is old data.
		const headers = new Headers(hit.headers);
		headers.set('X-From-Cache', '1');
		return new Response(hit.body, { status: hit.status, headers });
	}
}

interface PushMessage {
	title: string;
	body: string;
	region: number;
	tag: string;
	url: string;
}

self.addEventListener('push', (event) => {
	let msg: PushMessage;
	try {
		msg = event.data?.json() as PushMessage;
	} catch {
		return;
	}
	if (!msg?.title) return;
	event.waitUntil(
		self.registration.showNotification(msg.title, {
			body: msg.body,
			tag: msg.tag,
			icon: '/icons/icon-192.png',
			badge: '/icons/icon-192.png',
			lang: 'uk',
			data: { url: msg.url }
		})
	);
});

self.addEventListener('notificationclick', (event) => {
	event.notification.close();
	const target = new URL((event.notification.data?.url as string) ?? '/', self.location.origin).href;
	event.waitUntil(
		self.clients.matchAll({ type: 'window', includeUncontrolled: true }).then(async (wins) => {
			const win = wins.find((w) => new URL(w.url).origin === self.location.origin);
			if (win) {
				await win.focus();
				return win.navigate(target);
			}
			return self.clients.openWindow(target);
		})
	);
});
