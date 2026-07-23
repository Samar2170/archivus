<script lang="ts">
	import { onMount } from "svelte";
	import { goto } from "$app/navigation";
	import { page } from "$app/stores";
	import { authStore } from "$lib/stores/auth";
	import { getFiles, deleteFile, downloadFile } from "$lib/api/files";
	import type { FileMetaData } from "$lib/api/files";
	import Navbar from "$lib/components/Navbar.svelte";
	import FileCard from "$lib/components/FileCard.svelte";
	import Breadcrumbs from "$lib/components/Breadcrumbs.svelte";
	import FileFolderModal from "$lib/components/FileFolderModal.svelte";
	import FileContextMenu from "$lib/components/FileContextMenu.svelte";
	import MoveFileModal from "$lib/components/MoveFileModal.svelte";

	let files: FileMetaData[] = [];
	let loading = false;
	let error = "";

	// Pagination
	let currentPage = 1;
	let pageSize = 24;
	let total = 0;
	$: totalPages = Math.max(1, Math.ceil(total / pageSize));

	// Context menu (opened by right-click, or a long-press on touch devices).
	let menu: { file: FileMetaData; x: number; y: number } | null = null;
	// The file currently being moved / awaiting delete confirmation.
	let movingFile: FileMetaData | null = null;
	let deletingFile: FileMetaData | null = null;
	let deleteBusy = false;
	// Set when a long-press opens the menu, so the tap that follows on touch
	// devices doesn't also open the item.
	let suppressNextClick = false;
	let longPressTimer: ReturnType<typeof setTimeout> | null = null;

	$: currentFolder = $page.url.searchParams.get("folder") ?? "";

	async function loadFiles() {
		loading = true;
		error = "";
		try {
			const driveId = $authStore.driveId;
			if (!driveId) {
				error = "No drive available for this account.";
				return;
			}
			const result = await getFiles(
				currentFolder,
				driveId,
				currentPage,
				pageSize,
			);
			files = result.files ?? [];
			total = result.total ?? 0;
			pageSize = result.pageSize || pageSize;
			// If the page we asked for no longer exists (e.g. after deletes),
			// step back to the last valid page.
			const lastPage = Math.max(1, Math.ceil(total / pageSize));
			if (currentPage > lastPage) {
				currentPage = lastPage;
				await loadFiles();
			}
		} catch (err) {
			error = (err as Error).message;
		} finally {
			loading = false;
		}
	}

	function goToPage(target: number) {
		if (target < 1 || target > totalPages || target === currentPage) return;
		currentPage = target;
		loadFiles();
	}

	onMount(() => {
		if (!$authStore.isAuthenticated) {
			goto("/login");
			return;
		}
		loadFiles();
	});

	// Reload when folder query param changes, starting back at page 1
	$: if ($authStore.isAuthenticated && currentFolder !== undefined) {
		currentPage = 1;
		loadFiles();
	}

	function openItem(file: FileMetaData) {
		if (suppressNextClick) {
			suppressNextClick = false;
			return;
		}
		if (file.IsDir) {
			goto(
				`/?folder=${encodeURIComponent(file.NavigationPath || file.Path)}`,
			);
		} else if (file.SignedUrl) {
			window.open(file.SignedUrl, "_blank");
		}
	}

	// --- Context menu (right-click / long-press) ---

	function openMenu(e: MouseEvent, file: FileMetaData) {
		menu = { file, x: e.clientX, y: e.clientY };
	}

	function onPointerDown(e: PointerEvent, file: FileMetaData) {
		// Right-click is handled by `contextmenu`; here we add a long-press for
		// touch devices, which have no right mouse button.
		if (e.pointerType !== "touch") return;
		clearLongPress();
		longPressTimer = setTimeout(() => {
			suppressNextClick = true;
			menu = { file, x: e.clientX, y: e.clientY };
		}, 500);
	}

	function clearLongPress() {
		if (longPressTimer) {
			clearTimeout(longPressTimer);
			longPressTimer = null;
		}
	}

	async function handleMenuDownload(file: FileMetaData) {
		menu = null;
		const driveId = $authStore.driveId;
		if (!driveId) return;
		try {
			const blob = await downloadFile(file.ID, driveId);
			const url = URL.createObjectURL(blob);
			const a = document.createElement("a");
			a.href = url;
			a.download = file.Name;
			a.click();
			URL.revokeObjectURL(url);
		} catch (err) {
			alert("Download failed: " + (err as Error).message);
		}
	}

	async function confirmDelete() {
		if (!deletingFile) return;
		const driveId = $authStore.driveId;
		if (!driveId) return;
		deleteBusy = true;
		try {
			await deleteFile(deletingFile.ID, driveId);
			deletingFile = null;
			await loadFiles();
		} catch (err) {
			alert("Delete failed: " + (err as Error).message);
		} finally {
			deleteBusy = false;
		}
	}
</script>

<svelte:head>
	<title>Files — Archivus</title>
</svelte:head>

<div class="min-h-screen bg-gray-50">
	<Navbar />

	<main class="mx-auto max-w-7xl px-4 py-6 sm:px-6 lg:px-8">
		<!-- Breadcrumbs -->
		<div class="mb-4">
			<Breadcrumbs path={currentFolder} />
		</div>

		<!-- Content -->
		{#if loading}
			<div class="flex items-center justify-center py-24">
				<div
					class="h-8 w-8 animate-spin rounded-full border-4 border-orange-200 border-t-orange-600"
				/>
			</div>
		{:else if error}
			<div class="rounded-lg bg-red-50 p-4 text-sm text-red-700">
				{error}
			</div>
		{:else if files.length === 0}
			<div
				class="flex flex-col items-center justify-center py-24 text-gray-400"
			>
				<p class="text-lg font-medium">This folder is empty</p>
				<p class="text-sm">
					Upload files or create a folder to get started.
				</p>
			</div>
		{:else}
			<div
				class="grid grid-cols-2 gap-4 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6"
			>
				{#each files as file (file.ID)}
					<!-- svelte-ignore a11y-no-static-element-interactions -->
					<div
						on:click={() => openItem(file)}
						on:keydown={(e) => e.key === "Enter" && openItem(file)}
						on:contextmenu|preventDefault={(e) => openMenu(e, file)}
						on:pointerdown={(e) => onPointerDown(e, file)}
						on:pointerup={clearLongPress}
						on:pointermove={clearLongPress}
						on:pointerleave={clearLongPress}
						class="outline-none"
					>
						<FileCard {file} />
					</div>
				{/each}
			</div>

			<!-- Pagination -->
			{#if totalPages > 1}
				<div class="mt-6 flex items-center justify-center gap-4">
					<button
						on:click={() => goToPage(currentPage - 1)}
						disabled={currentPage <= 1}
						class="rounded-lg border border-gray-300 bg-white px-4 py-2 text-sm font-medium text-gray-700 hover:bg-gray-50 disabled:cursor-not-allowed disabled:opacity-50"
					>
						← Previous
					</button>
					<span class="text-sm text-gray-600">
						Page <span class="font-medium text-gray-900"
							>{currentPage}</span
						>
						of
						<span class="font-medium text-gray-900">{totalPages}</span>
						<span class="hidden text-gray-400 sm:inline"
							>· {total} items</span
						>
					</span>
					<button
						on:click={() => goToPage(currentPage + 1)}
						disabled={currentPage >= totalPages}
						class="rounded-lg border border-gray-300 bg-white px-4 py-2 text-sm font-medium text-gray-700 hover:bg-gray-50 disabled:cursor-not-allowed disabled:opacity-50"
					>
						Next →
					</button>
				</div>
			{/if}
		{/if}
	</main>

	<FileFolderModal {currentFolder} on:refresh={loadFiles} />

	<!-- Right-click / long-press context menu -->
	{#if menu}
		<FileContextMenu
			file={menu.file}
			x={menu.x}
			y={menu.y}
			on:close={() => (menu = null)}
			on:open={(e) => {
				menu = null;
				openItem(e.detail);
			}}
			on:download={(e) => handleMenuDownload(e.detail)}
			on:move={(e) => {
				movingFile = e.detail;
				menu = null;
			}}
			on:delete={(e) => {
				deletingFile = e.detail;
				menu = null;
			}}
		/>
	{/if}

	<!-- Move destination picker -->
	{#if movingFile}
		<MoveFileModal
			file={movingFile}
			on:close={() => (movingFile = null)}
			on:moved={() => {
				movingFile = null;
				loadFiles();
			}}
		/>
	{/if}

	<!-- Delete confirmation -->
	{#if deletingFile}
		<div
			class="fixed inset-0 z-[60] flex items-center justify-center bg-black/40 backdrop-blur-sm"
		>
			<div class="w-full max-w-sm rounded-2xl bg-white p-6 shadow-xl">
				<h2 class="mb-2 text-lg font-semibold text-gray-900">
					Delete file
				</h2>
				<p class="mb-5 text-sm text-gray-600">
					Move <span class="font-medium text-gray-800"
						>{deletingFile.Name}</span
					> to the recycle bin? It will be kept for 30 days before being permanently
					removed.
				</p>
				<div class="flex justify-end gap-2">
					<button
						on:click={() => (deletingFile = null)}
						disabled={deleteBusy}
						class="rounded-lg border border-gray-300 px-4 py-2 text-sm font-medium text-gray-700 hover:bg-gray-50 disabled:opacity-50"
					>
						Cancel
					</button>
					<button
						on:click={confirmDelete}
						disabled={deleteBusy}
						class="rounded-lg bg-red-600 px-4 py-2 text-sm font-medium text-white hover:bg-red-700 disabled:cursor-not-allowed disabled:opacity-50"
					>
						{deleteBusy ? "Deleting…" : "Delete"}
					</button>
				</div>
			</div>
		</div>
	{/if}
</div>
