<script lang="ts">
	import { onMount } from "svelte";
	import { goto } from "$app/navigation";
	import { page } from "$app/stores";
	import { authStore } from "$lib/stores/auth";
	import { getFiles } from "$lib/api/files";
	import type { FileMetaData } from "$lib/api/files";
	import Navbar from "$lib/components/Navbar.svelte";
	import FileCard from "$lib/components/FileCard.svelte";
	import Breadcrumbs from "$lib/components/Breadcrumbs.svelte";
	import FileFolderModal from "$lib/components/FileFolderModal.svelte";

	let files: FileMetaData[] = [];
	let loading = false;
	let error = "";

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
			const result = await getFiles(currentFolder, driveId);
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
						class="outline-none"
					>
						<FileCard {file} />
					</div>
				{/each}
			</div>
		{/if}
	</main>

	<FileFolderModal {currentFolder} on:refresh={loadFiles} />
</div>
