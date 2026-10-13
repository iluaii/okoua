// SPDX-License-Identifier: AGPL-3.0-or-later

/**
 * Shares a link through the system share sheet where there is one (phones), otherwise copies it.
 * Returns 'copied' so the caller can say so; 'shared' and 'cancelled' need no message.
 */
export async function shareLink(data: { title: string; text: string; url: string }): Promise<'shared' | 'copied' | 'cancelled' | 'failed'> {
	if (navigator.share) {
		try {
			await navigator.share(data);
			return 'shared';
		} catch (e) {
			if (e instanceof DOMException && e.name === 'AbortError') return 'cancelled';
			// Other errors (e.g. no permission in an iframe): fall back to copying.
		}
	}
	try {
		await navigator.clipboard.writeText(data.url);
		return 'copied';
	} catch {
		// Clipboard can be unavailable (insecure origin, permissions).
		return 'failed';
	}
}
