<script lang="ts">
	import { createEventDispatcher, onMount, onDestroy } from "svelte";
	import { FolderInput, Trash2, Download, FolderOpen } from "lucide-svelte";
	import type { FileMetaData } from "$lib/api/files";

	// Screen coordinates where the menu should appear (usually the cursor / touch
	// point) and the file the menu acts on.
	export let x = 0;
	export let y = 0;
	export let file: FileMetaData;

	const dispatch = createEventDispatcher<{
		open: FileMetaData;
		move: FileMetaData;
		delete: FileMetaData;
		download: FileMetaData;
		close: void;
	}>();

	let menuEl: HTMLDivElement;
	// Clamp the menu inside the viewport so it never overflows off-screen.
	let left = x;
	let top = y;

	function reposition() {
		if (!menuEl) return;
		const { width, height } = menuEl.getBoundingClientRect();
		const margin = 8;
		left = Math.min(x, window.innerWidth - width - margin);
		top = Math.min(y, window.innerHeight - height - margin);
		left = Math.max(margin, left);
		top = Math.max(margin, top);
	}

	function onKeydown(e: KeyboardEvent) {
		if (e.key === "Escape") dispatch("close");
	}

	onMount(() => {
		reposition();
		window.addEventListener("keydown", onKeydown);
		window.addEventListener("resize", () => dispatch("close"));
	});
	onDestroy(() => {
		window.removeEventListener("keydown", onKeydown);
	});
</script>

<!-- Backdrop: closes the menu on any outside click / right-click -->
<button
	class="fixed inset-0 z-50 cursor-default"
	aria-label="Close menu"
	on:click={() => dispatch("close")}
	on:contextmenu|preventDefault={() => dispatch("close")}
></button>

<div
	bind:this={menuEl}
	role="menu"
	tabindex="-1"
	class="fixed z-50 min-w-[11rem] overflow-hidden rounded-xl border border-gray-200 bg-white py-1 shadow-xl"
	style="left: {left}px; top: {top}px;"
>
	<p class="truncate px-3 py-1.5 text-xs font-medium text-gray-400" title={file.Name}>
		{file.Name}
	</p>
	<div class="my-1 h-px bg-gray-100"></div>

	<button
		role="menuitem"
		class="flex w-full items-center gap-2.5 px-3 py-2 text-sm text-gray-700 hover:bg-gray-50"
		on:click={() => dispatch("open", file)}
	>
		<FolderOpen class="h-4 w-4 text-gray-500" />
		{file.IsDir ? "Open folder" : "Open"}
	</button>

	{#if !file.IsDir}
		<button
			role="menuitem"
			class="flex w-full items-center gap-2.5 px-3 py-2 text-sm text-gray-700 hover:bg-gray-50"
			on:click={() => dispatch("download", file)}
		>
			<Download class="h-4 w-4 text-gray-500" />
			Download
		</button>

		<button
			role="menuitem"
			class="flex w-full items-center gap-2.5 px-3 py-2 text-sm text-gray-700 hover:bg-gray-50"
			on:click={() => dispatch("move", file)}
		>
			<FolderInput class="h-4 w-4 text-gray-500" />
			Move to…
		</button>

		<div class="my-1 h-px bg-gray-100"></div>

		<button
			role="menuitem"
			class="flex w-full items-center gap-2.5 px-3 py-2 text-sm text-red-600 hover:bg-red-50"
			on:click={() => dispatch("delete", file)}
		>
			<Trash2 class="h-4 w-4" />
			Delete
		</button>
	{/if}
</div>
