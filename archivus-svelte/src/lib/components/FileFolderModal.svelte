<script lang="ts">
	import { createEventDispatcher } from "svelte";
	import { Plus, Upload, FolderPlus, X } from "lucide-svelte";
	import { uploadFiles } from "$lib/api/files";
	import { createFolder } from "$lib/api/folder";

	export let currentFolder: string = "";

	const dispatch = createEventDispatcher<{ refresh: void }>();

	let fabOpen = false;
	let uploadOpen = false;
	let folderOpen = false;

	let selectedFiles: FileList | null = null;
	let uploadProgress = 0;
	let uploading = false;

	let newFolderName = "";
	let creatingFolder = false;

	async function handleUpload() {
		if (!selectedFiles?.length) return;
		uploading = true;
		uploadProgress = 0;
		try {
			await uploadFiles(
				selectedFiles,
				currentFolder,
				(p) => (uploadProgress = p),
			);
			uploadOpen = false;
			selectedFiles = null;
			uploadProgress = 0;
			dispatch("refresh");
		} catch (err) {
			alert("Upload failed: " + (err as Error).message);
		} finally {
			uploading = false;
		}
	}

	async function handleCreateFolder() {
		if (!newFolderName.trim()) return;
		creatingFolder = true;
		try {
			const fullPath = currentFolder
				? `${currentFolder}/${newFolderName.trim()}`
				: newFolderName.trim();
			await createFolder(fullPath);
			folderOpen = false;
			newFolderName = "";
			dispatch("refresh");
		} catch (err) {
			alert("Failed to create folder: " + (err as Error).message);
		} finally {
			creatingFolder = false;
		}
	}

	function closeAll() {
		fabOpen = false;
		uploadOpen = false;
		folderOpen = false;
	}
</script>

<!-- FAB -->
<div class="fixed bottom-6 right-6 z-40 flex flex-col items-end gap-3">
	{#if fabOpen}
		<!-- Upload option -->
		<button
			on:click={() => {
				uploadOpen = true;
				fabOpen = false;
			}}
			class="flex items-center gap-2 rounded-full bg-white px-4 py-2 text-sm font-medium text-gray-700
				shadow-lg ring-1 ring-gray-200 hover:bg-gray-50 transition-colors"
		>
			<Upload class="h-4 w-4 text-orange-500" />
			Upload files
		</button>

		<!-- New folder option -->
		<button
			on:click={() => {
				folderOpen = true;
				fabOpen = false;
			}}
			class="flex items-center gap-2 rounded-full bg-white px-4 py-2 text-sm font-medium text-gray-700
				shadow-lg ring-1 ring-gray-200 hover:bg-gray-50 transition-colors"
		>
			<FolderPlus class="h-4 w-4 text-orange-500" />
			New folder
		</button>
	{/if}

	<button
		on:click={() => (fabOpen = !fabOpen)}
		class="flex h-14 w-14 items-center justify-center rounded-full bg-orange-600 text-white
			shadow-lg hover:bg-orange-700 transition-all duration-200
			{fabOpen ? 'rotate-45' : 'rotate-0'}"
		aria-label="Actions"
	>
		<Plus class="h-7 w-7" />
	</button>
</div>

<!-- Click outside to close FAB -->
{#if fabOpen}
	<button
		class="fixed inset-0 z-30"
		aria-label="Close menu"
		on:click={() => (fabOpen = false)}
	></button>
{/if}

<!-- Upload Modal -->
{#if uploadOpen}
	<div
		class="fixed inset-0 z-50 flex items-center justify-center bg-black/40 backdrop-blur-sm"
	>
		<div class="w-full max-w-md rounded-2xl bg-white p-6 shadow-xl">
			<div class="mb-4 flex items-center justify-between">
				<h2 class="text-lg font-semibold text-gray-900">
					Upload files
				</h2>
				<button
					on:click={() => (uploadOpen = false)}
					class="text-gray-400 hover:text-gray-600"
				>
					<X class="h-5 w-5" />
				</button>
			</div>

			<input
				type="file"
				multiple
				on:change={(e) =>
					(selectedFiles = (e.target as HTMLInputElement).files)}
				class="w-full rounded-lg border border-gray-300 text-sm text-gray-600
					file:mr-3 file:rounded-md file:border-0 file:bg-orange-50 file:px-3 file:py-1.5
					file:text-sm file:font-medium file:text-orange-700 hover:file:bg-orange-100 mb-4"
			/>

			{#if uploading}
				<div class="mb-4">
					<div
						class="mb-1 flex justify-between text-xs text-gray-500"
					>
						<span>Uploading...</span>
						<span>{uploadProgress}%</span>
					</div>
					<div class="h-2 w-full rounded-full bg-gray-200">
						<div
							class="h-2 rounded-full bg-orange-500 transition-all"
							style="width: {uploadProgress}%"
						></div>
					</div>
				</div>
			{/if}

			<div class="flex justify-end gap-2">
				<button
					on:click={() => (uploadOpen = false)}
					disabled={uploading}
					class="rounded-lg border border-gray-300 px-4 py-2 text-sm font-medium text-gray-700
						hover:bg-gray-50 disabled:opacity-50"
				>
					Cancel
				</button>
				<button
					on:click={handleUpload}
					disabled={!selectedFiles?.length || uploading}
					class="rounded-lg bg-orange-600 px-4 py-2 text-sm font-medium text-white
						hover:bg-orange-700 disabled:opacity-50 disabled:cursor-not-allowed"
				>
					{uploading ? "Uploading..." : "Upload"}
				</button>
			</div>
		</div>
	</div>
{/if}

<!-- New Folder Modal -->
{#if folderOpen}
	<div
		class="fixed inset-0 z-50 flex items-center justify-center bg-black/40 backdrop-blur-sm"
	>
		<div class="w-full max-w-sm rounded-2xl bg-white p-6 shadow-xl">
			<div class="mb-4 flex items-center justify-between">
				<h2 class="text-lg font-semibold text-gray-900">New folder</h2>
				<button
					on:click={() => (folderOpen = false)}
					class="text-gray-400 hover:text-gray-600"
				>
					<X class="h-5 w-5" />
				</button>
			</div>

			<input
				type="text"
				bind:value={newFolderName}
				placeholder="Folder name"
				on:keydown={(e) => e.key === "Enter" && handleCreateFolder()}
				class="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm
					focus:border-orange-500 focus:outline-none focus:ring-1 focus:ring-orange-500 mb-4"
			/>

			<div class="flex justify-end gap-2">
				<button
					on:click={() => (folderOpen = false)}
					class="rounded-lg border border-gray-300 px-4 py-2 text-sm font-medium text-gray-700
						hover:bg-gray-50"
				>
					Cancel
				</button>
				<button
					on:click={handleCreateFolder}
					disabled={!newFolderName.trim() || creatingFolder}
					class="rounded-lg bg-orange-600 px-4 py-2 text-sm font-medium text-white
						hover:bg-orange-700 disabled:opacity-50 disabled:cursor-not-allowed"
				>
					{creatingFolder ? "Creating..." : "Create"}
				</button>
			</div>
		</div>
	</div>
{/if}
