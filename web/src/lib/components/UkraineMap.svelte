<!-- SPDX-License-Identifier: AGPL-3.0-or-later -->
<!-- Oblast map with zoom: mouse wheel, two-finger pinch, drag, buttons. -->
<script lang="ts">
	import { regions, viewBox as baseViewBox } from '#lib/map/regions.ts';
	import { placedOnRaions, placeNames, raionShapes, raionStatuses } from '#lib/map/places.svelte.ts';
	import type { RegionState } from '#lib/types.ts';

	interface Props {
		states: Map<number, RegionState>;
		selected: number | null;
		onselect: (uid: number) => void;
		/** Click outside any region (sea, background). */
		onclear?: () => void;
	}
	let { states, selected, onselect, onclear }: Props = $props();

	const [, , MAP_W, MAP_H] = baseViewBox.split(' ').map(Number);
	// Home view: the whole map with margins, plus room at the bottom for the menu that floats over the map.
	const PAD = 16;
	const HOME = { x: -PAD, y: -PAD, w: MAP_W + 2 * PAD, h: MAP_H + 2 * PAD + 90 };
	const BASE_W = HOME.w;
	const ASPECT = HOME.h / HOME.w;
	const MAX_ZOOM = 10;
	const DRAG_THRESHOLD = 6; // px: less than this is a tap, not a drag

	let svg = $state<SVGSVGElement>();
	let view = $state({ ...HOME });
	const zoom = $derived(BASE_W / view.w);

	const selectedRegion = $derived(regions.find((r) => r.uid === selected));
	// Partial alerts are drawn on their raions; an oblast whose alerts can't all be placed
	// (unknown raion, source without raions) is hatched as a whole instead.
	const raionState = $derived(raionStatuses(states.values()));
	const shapes = $derived(raionShapes());
	const alertRaions = $derived(shapes.raions.filter((r) => raionState.has(r.id)));

	/** Screen coordinates to map coordinates. */
	function toMap(clientX: number, clientY: number) {
		const ctm = svg!.getScreenCTM()!.inverse();
		const p = new DOMPoint(clientX, clientY).matrixTransform(ctm);
		return { x: p.x, y: p.y };
	}

	function setView(x: number, y: number, w: number) {
		w = Math.min(BASE_W, Math.max(BASE_W / MAX_ZOOM, w));
		const h = w * ASPECT;
		// Keep the view centre near the map
		// (a little slack lets southern regions be lifted above the sheet).
		const cx = Math.min(MAP_W + w / 4, Math.max(-w / 4, x + w / 2));
		const cy = Math.min(MAP_H + h / 3, Math.max(-h / 4, y + h / 2));
		view = { x: cx - w / 2, y: cy - h / 2, w, h };
	}

	/** Zoom around a point (map coordinates). */
	function zoomAt(px: number, py: number, factor: number) {
		const w = Math.min(BASE_W, Math.max(BASE_W / MAX_ZOOM, view.w / factor));
		const f = view.w / w;
		setView(px - (px - view.x) / f, py - (py - view.y) / f, w);
	}

	export function zoomBy(factor: number) {
		zoomAt(view.x + view.w / 2, view.y + view.h / 2, factor);
	}

	export function reset() {
		animateTo({ ...HOME });
	}

	/**
	 * Pans the map so the region is not hidden under a bottom panel bottomInset px tall.
	 * Does nothing if the region is already visible.
	 */
	export function reveal(uid: number, bottomInset: number) {
		const r = regions.find((r) => r.uid === uid);
		if (!r || !svg) return;
		const ctm = svg.getScreenCTM()!;
		const rect = svg.getBoundingClientRect();
		const y = ctm.f + r.cy * ctm.d; // region point on screen
		const visibleBottom = rect.bottom - bottomInset;
		if (y < visibleBottom - 40) return;
		const target = rect.top + (visibleBottom - rect.top) / 2;
		const dy = (y - target) / ctm.d;
		const w = view.w;
		const next = { ...view, y: view.y + dy };
		setView(next.x, next.y, w);
		const to = view;
		view = { ...next, y: next.y - dy }; // step back so we can animate
		animateTo(to);
	}

	let anim = 0;
	function animateTo(to: { x: number; y: number; w: number; h: number }) {
		cancelAnimationFrame(anim);
		const from = { ...view };
		const start = performance.now();
		const step = (t: number) => {
			const k = Math.min(1, (t - start) / 250);
			const e = 1 - (1 - k) ** 3;
			view = {
				x: from.x + (to.x - from.x) * e,
				y: from.y + (to.y - from.y) * e,
				w: from.w + (to.w - from.w) * e,
				h: from.h + (to.h - from.h) * e
			};
			if (k < 1) anim = requestAnimationFrame(step);
		};
		anim = requestAnimationFrame(step);
	}

	function onwheel(e: WheelEvent) {
		e.preventDefault();
		cancelAnimationFrame(anim);
		const p = toMap(e.clientX, e.clientY);
		zoomAt(p.x, p.y, Math.exp(-e.deltaY * (e.deltaMode === 1 ? 0.05 : 0.0015)));
	}

	// Active touches/pointers and gesture state.
	const pointers = new Map<number, { x: number; y: number }>();
	let downAt = { x: 0, y: 0 };
	let dragged = false;
	let pinchDist = 0;

	function onpointerdown(e: PointerEvent) {
		cancelAnimationFrame(anim);
		pointers.set(e.pointerId, { x: e.clientX, y: e.clientY });
		if (pointers.size === 1) {
			downAt = { x: e.clientX, y: e.clientY };
			dragged = false;
		} else if (pointers.size === 2) {
			pinchDist = distance();
			dragged = true;
		}
	}

	function onpointermove(e: PointerEvent) {
		const prev = pointers.get(e.pointerId);
		if (!prev) return;
		const next = { x: e.clientX, y: e.clientY };

		if (pointers.size === 1) {
			if (!dragged && Math.hypot(next.x - downAt.x, next.y - downAt.y) < DRAG_THRESHOLD) return;
			if (!dragged) {
				dragged = true;
				// Capture only once a drag starts; otherwise the click would go to the svg instead of the region.
				svg!.setPointerCapture(e.pointerId);
			}
			const a = toMap(prev.x, prev.y);
			const b = toMap(next.x, next.y);
			setView(view.x - (b.x - a.x), view.y - (b.y - a.y), view.w);
		} else if (pointers.size === 2) {
			const before = midpoint();
			pointers.set(e.pointerId, next);
			const after = midpoint();
			const d = distance();
			const a = toMap(before.x, before.y);
			const b = toMap(after.x, after.y);
			setView(view.x - (b.x - a.x), view.y - (b.y - a.y), view.w);
			if (pinchDist > 0) {
				const m = toMap(after.x, after.y);
				zoomAt(m.x, m.y, d / pinchDist);
			}
			pinchDist = d;
			return;
		}
		pointers.set(e.pointerId, next);
	}

	function onpointerup(e: PointerEvent) {
		pointers.delete(e.pointerId);
		if (pointers.size < 2) pinchDist = 0;
	}

	function distance() {
		const [a, b] = [...pointers.values()];
		return Math.hypot(a.x - b.x, a.y - b.y);
	}

	function midpoint() {
		const [a, b] = [...pointers.values()];
		return { x: (a.x + b.x) / 2, y: (a.y + b.y) / 2 };
	}

	function onclickRegion(uid: number) {
		if (!dragged) onselect(uid);
	}

	function onclickBackground(e: MouseEvent) {
		if (!dragged && e.target === svg) onclear?.();
	}

	function label(name: string, state: RegionState | undefined) {
		if (!state) return `${name}: тривоги немає`;
		if (state.status === 'full') return `${name}: тривога`;
		const places = placeNames(state);
		return places.length ? `${name}: тривога, ${places.join(', ')}` : `${name}: тривога в частині області`;
	}

	function onkeydown(e: KeyboardEvent, uid: number) {
		if (e.key === 'Enter' || e.key === ' ') {
			e.preventDefault();
			onselect(uid);
		}
	}
</script>

<!-- Gestures and background clicks are pointer-only; keyboard users pick regions with Tab/Enter and close the sheet with Esc. -->
<!-- svelte-ignore a11y_click_events_have_key_events, a11y_no_noninteractive_element_interactions -->
<svg
	bind:this={svg}
	viewBox="{view.x} {view.y} {view.w} {view.h}"
	preserveAspectRatio="xMidYMid meet"
	class="map"
	class:zoomed={zoom > 1.01}
	role="group"
	aria-label="Карта тривог України"
	{onwheel}
	{onpointerdown}
	{onpointermove}
	{onpointerup}
	onpointercancel={onpointerup}
	onclick={onclickBackground}
>
	<defs>
		<pattern id="hatch-partial" width="8" height="8" patternUnits="userSpaceOnUse" patternTransform="rotate(45)">
			<rect width="8" height="8" class="hatch-bg" />
			<rect width="4" height="8" class="hatch-line" />
		</pattern>
	</defs>
	{#each regions as r (r.uid)}
		{@const state = states.get(r.uid)}
		<path
			d={r.d}
			class="region"
			class:full={state?.status === 'full'}
			class:partial={state?.status === 'partial' && !placedOnRaions(state)}
			role="button"
			tabindex="0"
			aria-label={label(r.name, state)}
			aria-pressed={selected === r.uid}
			onclick={() => onclickRegion(r.uid)}
			onkeydown={(e) => onkeydown(e, r.uid)}
		>
			<title>{r.name}</title>
		</path>
	{/each}
	<!-- Raions under alert, borders between raions and oblast borders on top; taps go through to the oblast. -->
	{#each alertRaions as r (r.id)}
		<path d={r.d} class="raion" class:partial={raionState.get(r.id) === 'partial'} aria-hidden="true" />
	{/each}
	{#if shapes.mesh}<path d={shapes.mesh} class="mesh" aria-hidden="true" />{/if}
	{#each regions as r (r.uid)}
		<path d={r.d} class="border" aria-hidden="true" />
	{/each}
	{#if selectedRegion}
		<!-- Outline of the selected region, drawn above its neighbours. -->
		<path d={selectedRegion.d} class="outline" aria-hidden="true" />
	{/if}
</svg>

<style>
	.map {
		display: block;
		width: 100%;
		height: 100%;
		touch-action: none;
		user-select: none;
		-webkit-user-select: none;
		-webkit-tap-highlight-color: transparent;
		cursor: grab;
	}
	.map:active {
		cursor: grabbing;
	}
	.region {
		fill: var(--map-calm);
		stroke: var(--map-border);
		stroke-width: 1.2;
		stroke-linejoin: round;
		vector-effect: non-scaling-stroke;
		cursor: pointer;
		transition: fill 0.3s;
		outline: none;
	}
	.region:hover {
		fill: var(--map-calm-hover);
	}
	.region.full {
		fill: var(--alert);
	}
	.region.full:hover {
		fill: var(--alert-hover);
	}
	.region.partial {
		fill: url(#hatch-partial);
	}
	.raion {
		fill: var(--alert);
		pointer-events: none;
	}
	.raion.partial {
		fill: url(#hatch-partial);
	}
	.mesh {
		fill: none;
		stroke: var(--map-border);
		stroke-width: 0.6;
		stroke-linejoin: round;
		stroke-opacity: 0.55;
		vector-effect: non-scaling-stroke;
		pointer-events: none;
	}
	.border {
		fill: none;
		stroke: var(--map-border);
		stroke-width: 1.2;
		stroke-linejoin: round;
		vector-effect: non-scaling-stroke;
		pointer-events: none;
	}
	.region:focus-visible {
		stroke: var(--focus);
		stroke-width: 3;
	}
	.hatch-bg {
		fill: var(--map-calm);
	}
	.hatch-line {
		fill: var(--alert);
	}
	.outline {
		fill: none;
		stroke: var(--text);
		stroke-width: 2.5;
		stroke-linejoin: round;
		vector-effect: non-scaling-stroke;
		pointer-events: none;
	}
</style>
