// SPDX-License-Identifier: AGPL-3.0-or-later

// "Install the app" support: Chromium browsers fire beforeinstallprompt, which we keep
// so a button can show the install dialog later. Other browsers install from their menu.

interface InstallPromptEvent extends Event {
	prompt(): Promise<void>;
	userChoice: Promise<{ outcome: 'accepted' | 'dismissed' }>;
}

class Install {
	/** The browser offered installation and we can show its dialog. */
	available = $state(false);
	/** Running as an installed app (home screen / standalone window). */
	installed = $state(false);

	#event: InstallPromptEvent | null = null;

	init() {
		this.installed =
			matchMedia('(display-mode: standalone)').matches ||
			(navigator as Navigator & { standalone?: boolean }).standalone === true;
		window.addEventListener('beforeinstallprompt', (e) => {
			e.preventDefault();
			this.#event = e as InstallPromptEvent;
			this.available = true;
		});
		window.addEventListener('appinstalled', () => {
			this.installed = true;
			this.available = false;
		});
	}

	async prompt() {
		if (!this.#event) return;
		await this.#event.prompt();
		await this.#event.userChoice;
		this.#event = null;
		this.available = false;
	}
}

export const install = new Install();
