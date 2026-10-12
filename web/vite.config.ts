// SPDX-License-Identifier: AGPL-3.0-or-later
import adapter from '@sveltejs/adapter-static';
import { sveltekit } from '@sveltejs/kit/vite';
import { defineConfig } from 'vite';

export default defineConfig({
	plugins: [
		sveltekit({
			// The site is fully static; data comes from the Go server via /api.
			adapter: adapter({ pages: 'build', assets: 'build', fallback: undefined, strict: true })
		})
	],
	server: {
		// In development the API is proxied to the Go server.
		proxy: { '/api': 'http://127.0.0.1:8080' }
	}
});
