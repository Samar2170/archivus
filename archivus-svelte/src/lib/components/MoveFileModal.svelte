<script lang="ts">
	import { createEventDispatcher, onMount } from "svelte";
	import { X, Folder, ChevronRight, CornerLeftUp, Loader2 } from "lucide-svelte";
	import { getFiles, moveFile, type FileMetaData } from "$lib/api/files";
	import { authStore } from "$lib/stores/auth";

	// The file being moved.
	export let file: FileMetaData;

	const dispatch = createEventDispatcher<{ moved: void; close: void }>();

	// browsePath is the destination folder currently being viewed, relative to
	// the drive root ("" = root).
	let browsePath = "";
	let folders: FileMetaData[] = [];
	let loading = false;
	let moving = false;
	let error = "";

	async function loadFolders() {
		const driveId = $authStore.driveId;
		if (!driveId) {
			error = "No drive available for this account.";
			return;
		}
		loading = true;
		error = "";
		try {
			// Directories sort before files, so a max-size page (500) is enough
			// to list every folder for the destination picker.
			const result = await getFiles(browsePath, driveId, { page: 1, pageSize: 500 });
			folders = (result.files ?? []).filter((f) => f.IsDir);
		} catch (err) {
			error = (err as Error).message;
		} finally {
			loading = false;
		}
	}

	function enterFolder(folder: FileMetaData) {
		browsePath = folder.NavigationPath || folder.Path;
		loadFolders();
	}

	function goUp() {
		const idx = browsePath.lastIndexOf("/");
		browsePath = idx === -1 ? "" : browsePath.slice(0, idx);
		loadFolders();
	}

	async function confirmMove() {
		const driveId = $authStore.driveId;
		if (!driveId) return;
		moving = true;
		error = "";
		try {
			await moveFile(file.ID, browsePath, driveId);
			dispatch("moved");
		} catch (err) {
			error = (err as Error).message;
		} finally {
			moving = false;
		}
	}

	onMount(loadFolders);
</script>

<div class="fixed inset-0 z-[60] flex items-center justify-center bg-black/40 backdrop-blur-sm">
	<div class="flex w-full max-w-md flex-col rounded-2xl bg-white p-6 shadow-xl">
		<div class="mb-4 flex items-center justify-between">
			<h2 class="text-lg font-semibold text-gray-900">Move file</h2>
			<button on:click={() => dispatch("close")} class="text-gray-400 hover:text-gray-600">
				<X class="h-5 w-5" />
			</button>
		</div>

		<p class="mb-3 truncate text-sm text-gray-500" title={file.Name}>
			Moving <span class="font-medium text-gray-700">{file.Name}</span>
		</p>

		<!-- Current destination path -->
		<div class="mb-2 flex items-center gap-2 rounded-lg bg-gray-50 px-3 py-2 text-sm">
			<Folder class="h-4 w-4 shrink-0 text-orange-500" />
			<span class="truncate text-gray-700">{browsePath || "Home"}</span>
		</div>

		<!-- Folder list -->
		<div class="mb-4 h-56 overflow-y-auto rounded-lg border border-gray-200">
			{#if browsePath}
				<button
					class="flex w-full items-center gap-2 border-b border-gray-100 px-3 py-2 text-sm text-gray-600 hover:bg-gray-50"
					on:click={goUp}
				>
					<CornerLeftUp class="h-4 w-4" />
					Up one level
				</button>
			{/if}

			{#if loading}
				<div class="flex items-center justify-center py-10 text-gray-400">
					<Loader2 class="h-5 w-5 animate-spin" />
				</div>
			{:else if folders.length === 0}
				<p class="px-3 py-10 text-center text-sm text-gray-400">No sub-folders here</p>
			{:else}
				{#each folders as folder (folder.ID)}
					<button
						class="flex w-full items-center justify-between px-3 py-2 text-sm text-gray-700 hover:bg-gray-50"
						on:click={() => enterFolder(folder)}
					>
						<span class="flex items-center gap-2 truncate">
							<Folder class="h-4 w-4 shrink-0 text-gray-500" />
							<span class="truncate">{folder.Name}</span>
						</span>
						<ChevronRight class="h-4 w-4 shrink-0 text-gray-300" />
					</button>
				{/each}
			{/if}
		</div>

		{#if error}
			<p class="mb-3 rounded-lg bg-red-50 px-3 py-2 text-sm text-red-700">{error}</p>
		{/if}

		<div class="flex justify-end gap-2">
			<button
				on:click={() => dispatch("close")}
				disabled={moving}
				class="rounded-lg border border-gray-300 px-4 py-2 text-sm font-medium text-gray-700 hover:bg-gray-50 disabled:opacity-50"
			>
				Cancel
			</button>
			<button
				on:click={confirmMove}
				disabled={moving}
				class="rounded-lg bg-orange-600 px-4 py-2 text-sm font-medium text-white hover:bg-orange-700 disabled:cursor-not-allowed disabled:opacity-50"
			>
				{moving ? "Moving…" : browsePath ? `Move to "${browsePath.split("/").pop()}"` : "Move to Home"}
			</button>
		</div>
	</div>
</div>
