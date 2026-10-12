<!-- SPDX-License-Identifier: AGPL-3.0-or-later -->
<!-- Bottom sheet over the content, above the menu. Same width as the app column. -->
<script lang="ts">
	import type { Snippet } from 'svelte';
	import { fly } from 'svelte/transition';
	import { cubicOut } from 'svelte/easing';

	interface Props {
		label: string;
		onclose: () => void;
		/** Sheet height, so the page can react to it. */
		height?: number;
		children: Snippet;
	}
	let { label, onclose, height = $bindable(0), children }: Props = $props();

	function onkeydown(e: KeyboardEvent) {
		if (e.key === 'Escape') onclose();
	}
</script>

<svelte:window {onkeydown} />

<aside
	class="sheet"
	aria-label={label}
	bind:clientHeight={height}
	transition:fly={{ y: 40, duration: 220, easing: cubicOut }}
>
	<div class="grip" aria-hidden="true"></div>
	{@render children()}
</aside>

<style>
	.sheet {
		position: fixed;
		left: 50%;
		bottom: calc(var(--nav-h) + 20px + env(safe-area-inset-bottom));
		z-index: 15;
		transform: translateX(-50%);
		width: min(calc(100% - 2 * var(--gutter)), calc(var(--column) - 2 * var(--gutter)));
		max-height: min(60dvh, 520px);
		overflow-y: auto;
		overscroll-behavior: contain;
		padding: 8px 18px 16px;
		border-radius: var(--radius);
		background: var(--surface);
		border: 1px solid var(--border);
		box-shadow: var(--shadow);
	}
	.grip {
		width: 36px;
		height: 4px;
		margin: 0 auto 8px;
		border-radius: 2px;
		background: var(--border);
	}
</style>
