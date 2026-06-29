<script lang="ts">
	import { onMount } from "svelte";
	import { goto } from "$app/navigation";
	import { page } from "$app/stores";
	import { authStore } from "$lib/stores/auth";
	import { getFiles, moveFile } from "$lib/api/files";
	import type { FileMetaData } from "$lib/api/files";
	import Navbar from "$lib/components/Navbar.svelte";
	import FileCard from "$lib/components/FileCard.svelte";
	import Breadcrumbs from "$lib/components/Breadcrumbs.svelte";
	import FileFolderModal from "$lib/components/FileFolderModal.svelte";
	import { dndzone } from "svelte-dnd-action";
	import { flipDurationMs } from "$lib/utils/dnd";

	let files: FileMetaData[] = [];
	let loading = false;
	let error = "";
	let dragOverFolder: string | null = null;

	$: currentFolder = $page.url.searchParams.get("folder") ?? "";

	async function loadFiles() {
		loading = true;
		error = "";
		try {
			const result = await getFiles(currentFolder);
			files = result.files ?? [];
		} catch (err) {
			error = (err as Error).message;
		} finally {
			loading = false;
		}
	}

	onMount(() => {
		if (!$authStore.isAuthenticated) {
			goto("/login");
			return;
		}
		loadFiles();
	});

	// Reload when folder query param changes
	$: if ($authStore.isAuthenticated && currentFolder !== undefined) {
		loadFiles();
	}

	function openItem(file: FileMetaData) {
		if (file.IsDir) {
			goto(
				`/?folder=${encodeURIComponent(file.NavigationPath || file.Path)}`,
			);
		} else if (file.SignedUrl) {
			window.open(file.SignedUrl, "_blank");
		}
	}

	// Drag state for moving files into folders
	let draggedFileId: string | null = null;

	function handleDragStart(fileId: string) {
		draggedFileId = fileId;
	}

	function handleDragEnd() {
		draggedFileId = null;
		dragOverFolder = null;
	}

	async function handleDropOnFolder(targetFile: FileMetaData) {
		if (!draggedFileId || !targetFile.IsDir) return;
		const draggedFile = files.find(
			(f) => f.id === draggedFileId || f.ID === draggedFileId,
		);
		if (!draggedFile || draggedFile.ID === targetFile.ID) return;

		try {
			await moveFile(
				draggedFile.Path,
				targetFile.NavigationPath || targetFile.Path,
			);
			await loadFiles();
		} catch (err) {
			alert("Move failed: " + (err as Error).message);
		} finally {
			draggedFileId = null;
			dragOverFolder = null;
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
				{#each files as file (file.ID || file.id)}
					<!-- svelte-ignore a11y-no-static-element-interactions -->
					<div
						draggable="true"
						on:dragstart={() => handleDragStart(file.ID || file.id)}
						on:dragend={handleDragEnd}
						on:dragover|preventDefault={() => {
							if (file.IsDir) dragOverFolder = file.ID || file.id;
						}}
						on:dragleave={() => {
							if (dragOverFolder === (file.ID || file.id))
								dragOverFolder = null;
						}}
						on:drop|preventDefault={() => handleDropOnFolder(file)}
						on:click={() => openItem(file)}
						on:keydown={(e) => e.key === "Enter" && openItem(file)}
						class="outline-none
							{dragOverFolder === (file.ID || file.id)
							? 'ring-2 ring-orange-400 rounded-xl'
							: ''}"
					>
						<FileCard
							{file}
							dragging={draggedFileId === (file.ID || file.id)}
						/>
					</div>
				{/each}
			</div>
		{/if}
	</main>

	<FileFolderModal {currentFolder} on:refresh={loadFiles} />
</div>
