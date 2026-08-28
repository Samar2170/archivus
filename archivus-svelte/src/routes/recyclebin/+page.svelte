<script lang="ts">
	import { onMount } from "svelte";
	import { goto } from "$app/navigation";
	import { authStore } from "$lib/stores/auth";
	import {
		getRecycleBin,
		restoreFile,
		purgeRecycleBinItem,
		type RecycleEntry
	} from "$lib/api/files";
	import Navbar from "$lib/components/Navbar.svelte";
	import { RotateCcw, Trash2, Loader2, Folder, File } from "lucide-svelte";

	let items: RecycleEntry[] = [];
	let loading = false;
	let error = "";
	// IDs currently being restored, to disable their row buttons.
	let restoring = new Set<string>();
	// Item awaiting a "delete forever" confirmation, and whether it's running.
	let purgingItem: RecycleEntry | null = null;
	let purgeBusy = false;

	async function load() {
		loading = true;
		error = "";
		try {
			const driveId = $authStore.driveId;
			if (!driveId) {
				error = "No drive available for this account.";
				return;
			}
			const result = await getRecycleBin(driveId);
			items = result.items ?? [];
		} catch (err) {
			error = (err as Error).message;
		} finally {
			loading = false;
		}
	}

	async function handleRestore(item: RecycleEntry) {
		const driveId = $authStore.driveId;
		if (!driveId) return;
		restoring = new Set(restoring).add(item.ID);
		try {
			await restoreFile(item.ID, driveId);
			items = items.filter((i) => i.ID !== item.ID);
		} catch (err) {
			alert("Restore failed: " + (err as Error).message);
		} finally {
			const next = new Set(restoring);
			next.delete(item.ID);
			restoring = next;
		}
	}

	// handlePurge permanently deletes the confirmed item right away, skipping
	// the remainder of its 30 day retention window. This cannot be undone.
	async function confirmPurge() {
		const item = purgingItem;
		const driveId = $authStore.driveId;
		if (!item || !driveId) return;
		purgeBusy = true;
		try {
			await purgeRecycleBinItem(item.ID, driveId);
			items = items.filter((i) => i.ID !== item.ID);
			purgingItem = null;
		} catch (err) {
			alert("Delete forever failed: " + (err as Error).message);
		} finally {
			purgeBusy = false;
		}
	}

	onMount(() => {
		if (!$authStore.isAuthenticated) {
			goto("/login");
			return;
		}
		load();
	});

	function formatSize(mb: number): string {
		if (mb < 1) return `${(mb * 1024).toFixed(0)} KB`;
		return `${mb.toFixed(1)} MB`;
	}

	function formatDate(iso: string): string {
		const d = new Date(iso);
		if (isNaN(d.getTime())) return "—";
		return d.toLocaleDateString(undefined, {
			year: "numeric",
			month: "short",
			day: "numeric",
		});
	}

	// Whole days from now until the item is permanently purged (floored at 0).
	function daysLeft(iso: string): number {
		const d = new Date(iso).getTime();
		if (isNaN(d)) return 0;
		return Math.max(0, Math.ceil((d - Date.now()) / 86_400_000));
	}
</script>

<svelte:head>
	<title>Recycle Bin — Archivus</title>
</svelte:head>

<div class="min-h-screen bg-gray-50">
	<Navbar />

	<main class="mx-auto max-w-7xl px-4 py-6 sm:px-6 lg:px-8">
		<div class="mb-4 flex items-center gap-2">
			<Trash2 class="h-5 w-5 text-gray-500" />
			<h1 class="text-xl font-semibold text-gray-900">Recycle Bin</h1>
		</div>
		<p class="mb-6 text-sm text-gray-500">
			Deleted files are kept here for 30 days, then permanently removed.
		</p>

		{#if loading}
			<div class="flex items-center justify-center py-24">
				<div
					class="h-8 w-8 animate-spin rounded-full border-4 border-orange-200 border-t-orange-600"
				></div>
			</div>
		{:else if error}
			<div class="rounded-lg bg-red-50 p-4 text-sm text-red-700">
				{error}
			</div>
		{:else if items.length === 0}
			<div
				class="flex flex-col items-center justify-center py-24 text-gray-400"
			>
				<Trash2 class="mb-3 h-10 w-10" />
				<p class="text-lg font-medium">The recycle bin is empty</p>
				<p class="text-sm">Deleted files will appear here.</p>
			</div>
		{:else}
			<div class="overflow-hidden rounded-xl border border-gray-200 bg-white shadow-sm">
				<!-- Header (desktop) -->
				<div
					class="hidden grid-cols-12 gap-4 border-b border-gray-100 px-4 py-3 text-xs font-medium uppercase tracking-wide text-gray-400 sm:grid"
				>
					<div class="col-span-4">Name</div>
					<div class="col-span-2">Size</div>
					<div class="col-span-2">Deleted</div>
					<div class="col-span-2">Purges in</div>
					<div class="col-span-2"></div>
				</div>

				{#each items as item (item.ID)}
					<div
						class="grid grid-cols-2 items-center gap-x-4 gap-y-1 border-b border-gray-50 px-4 py-3 last:border-b-0 sm:grid-cols-12"
					>
						<div class="col-span-2 min-w-0 sm:col-span-4">
							<p
								class="flex items-center gap-1.5 truncate text-sm font-medium text-gray-800"
								title={item.Name}
							>
								{#if item.IsDir}
									<Folder class="h-4 w-4 shrink-0 text-orange-500" />
								{:else}
									<File class="h-4 w-4 shrink-0 text-gray-400" />
								{/if}
								<span class="truncate">{item.Name}</span>
							</p>
							<p
								class="truncate text-xs text-gray-400"
								title={item.OriginalPath}
							>
								{item.OriginalPath}
							</p>
						</div>
						<div class="text-sm text-gray-500 sm:col-span-2">
							{formatSize(item.Size)}
						</div>
						<div class="text-sm text-gray-500 sm:col-span-2">
							{formatDate(item.DeletedAt)}
						</div>
						<div class="sm:col-span-2">
							<span
								class="inline-flex rounded-full bg-gray-100 px-2 py-0.5 text-xs font-medium
									{daysLeft(item.ExpiresAt) <= 3
									? 'text-red-600'
									: 'text-gray-600'}"
							>
								{daysLeft(item.ExpiresAt)} day{daysLeft(item.ExpiresAt) === 1 ? "" : "s"}
							</span>
						</div>
						<div class="col-span-2 flex justify-start gap-2 sm:col-span-2 sm:justify-end">
							<button
								on:click={() => handleRestore(item)}
								disabled={restoring.has(item.ID) || purgeBusy}
								class="flex items-center gap-1.5 rounded-lg border border-gray-300 px-3 py-1.5 text-sm
									font-medium text-gray-700 hover:bg-gray-50 disabled:cursor-not-allowed disabled:opacity-50"
								title="Restore to original location"
							>
								{#if restoring.has(item.ID)}
									<Loader2 class="h-4 w-4 animate-spin" />
								{:else}
									<RotateCcw class="h-4 w-4" />
								{/if}
								Restore
							</button>
							<button
								on:click={() => (purgingItem = item)}
								disabled={restoring.has(item.ID) || purgeBusy}
								class="flex items-center gap-1.5 rounded-lg border border-red-300 px-3 py-1.5 text-sm
									font-medium text-red-600 hover:bg-red-50 disabled:cursor-not-allowed disabled:opacity-50"
								title="Delete permanently without waiting for the retention window"
							>
								<Trash2 class="h-4 w-4" />
								Delete forever
							</button>
						</div>
					</div>
				{/each}
			</div>
		{/if}
	</main>

	<!-- Delete forever confirmation -->
	{#if purgingItem}
		<div
			class="fixed inset-0 z-[60] flex items-center justify-center bg-black/40 backdrop-blur-sm"
		>
			<div class="w-full max-w-sm rounded-2xl bg-white p-6 shadow-xl">
				<h2 class="mb-2 text-lg font-semibold text-gray-900">
					Delete {purgingItem.IsDir ? "folder" : "file"} forever
				</h2>
				<p class="mb-5 text-sm text-gray-600">
					Permanently delete
					<span class="font-medium text-gray-800">{purgingItem.Name}</span>
					{purgingItem.IsDir ? "and everything inside it" : ""} right now? This
					cannot be undone.
				</p>
				<div class="flex justify-end gap-2">
					<button
						on:click={() => (purgingItem = null)}
						disabled={purgeBusy}
						class="rounded-lg border border-gray-300 px-4 py-2 text-sm font-medium text-gray-700 hover:bg-gray-50 disabled:opacity-50"
					>
						Cancel
					</button>
					<button
						on:click={confirmPurge}
						disabled={purgeBusy}
						class="rounded-lg bg-red-600 px-4 py-2 text-sm font-medium text-white hover:bg-red-700 disabled:cursor-not-allowed disabled:opacity-50"
					>
						{purgeBusy ? "Deleting…" : "Delete forever"}
					</button>
				</div>
			</div>
		</div>
	{/if}
</div>
