<script lang="ts">
	import { onMount } from "svelte";
	import { goto } from "$app/navigation";
	import { page } from "$app/stores";
	import { authStore } from "$lib/stores/auth";
	import {
		getSharedRoots,
		listSharedFiles,
		downloadSharedFile,
		type SharedRoot,
	} from "$lib/api/shared";
	import type { FileMetaData } from "$lib/api/files";
	import {
		fileCategories,
		sortByOptions,
		type FileCategory,
		type SortBy,
		type SortOrder,
	} from "$lib/data/constants";
	import { Loader2, Folder, Share2, ArrowLeft, ArrowDown, ArrowUp, ListFilter } from "lucide-svelte";
	import Navbar from "$lib/components/Navbar.svelte";
	import FileCard from "$lib/components/FileCard.svelte";
	import FileViewerModal from "$lib/components/FileViewerModal.svelte";

	let roots: SharedRoot[] = [];
	let loading = false;
	let error = "";

	// Browsing state, mirrored into query params:
	//   ?drive=<id>&root=<shared root>&folder=<drive-relative path>
	$: driveId = $page.url.searchParams.get("drive") ?? "";
	$: rootPath = $page.url.searchParams.get("root") ?? "";
	$: folderPath = $page.url.searchParams.get("folder") ?? rootPath;

	// Active root record (for the access badge and drive name in the header).
	$: activeRoot = roots.find((r) => r.driveId === driveId && r.rootPath === rootPath) ?? null;

	// Clamp navigation to the shared subtree: the server enforces this too, but
	// a stray URL should land on the root instead of showing an error.
	$: safeFolderPath =
		folderPath === rootPath || folderPath.startsWith(rootPath + "/") ? folderPath : rootPath;

	let files: FileMetaData[] = [];
	let listLoading = false;
	let listError = "";

	// Filtering & sorting
	let category: FileCategory | "" = "";
	let sortBy: SortBy = "name";
	let sortOrder: SortOrder = "asc";

	// Pagination
	let currentPage = 1;
	let pageSize = 24;
	let total = 0;
	$: totalPages = Math.max(1, Math.ceil(total / pageSize));

	// The file currently shown in the viewer modal (null = closed).
	let viewerFile: FileMetaData | null = null;

	async function loadRoots() {
		loading = true;
		error = "";
		try {
			roots = await getSharedRoots();
		} catch (err) {
			error = (err as Error).message;
		} finally {
			loading = false;
		}
	}

	async function loadFiles() {
		if (!driveId || !rootPath) return;
		listLoading = true;
		listError = "";
		try {
			const result = await listSharedFiles(
				driveId,
				rootPath,
				safeFolderPath,
				currentPage,
				pageSize,
				{ category, sortBy, sortOrder }
			);
			files = result.files ?? [];
			total = result.total ?? 0;
			pageSize = result.pageSize || pageSize;
			const lastPage = Math.max(1, Math.ceil(total / pageSize));
			if (currentPage > lastPage) {
				currentPage = lastPage;
				await loadFiles();
			}
		} catch (err) {
			listError = (err as Error).message;
			files = [];
		} finally {
			listLoading = false;
		}
	}

	function openRoot(root: SharedRoot) {
		goto(
			`/shared?drive=${encodeURIComponent(root.driveId)}&root=${encodeURIComponent(root.rootPath)}&folder=${encodeURIComponent(root.rootPath)}`
		);
	}

	function backToRoots() {
		goto("/shared");
	}

	function navigate(targetPath: string) {
		goto(
			`/shared?drive=${encodeURIComponent(driveId)}&root=${encodeURIComponent(rootPath)}&folder=${encodeURIComponent(targetPath)}`
		);
	}

	function openItem(file: FileMetaData) {
		if (file.IsDir) {
			navigate(file.NavigationPath || file.Path);
		} else {
			viewerFile = file;
		}
	}

	async function fetchSharedBlob(file: FileMetaData): Promise<Blob> {
		return downloadSharedFile(file.ID, driveId, rootPath);
	}

	// Fetch the file as a blob and trigger a browser download, shared by the
	// card download icon and the viewer's download CTA.
	async function downloadBlob(file: FileMetaData) {
		try {
			const blob = await fetchSharedBlob(file);
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

	function goToPage(target: number) {
		if (target < 1 || target > totalPages || target === currentPage) return;
		currentPage = target;
		loadFiles();
	}

	function applyFilter(nextCategory: FileCategory | "") {
		if (category === nextCategory) return;
		category = nextCategory;
		currentPage = 1;
		loadFiles();
	}

	function applySort(nextSortBy: SortBy) {
		if (sortBy === nextSortBy) return;
		sortBy = nextSortBy;
		currentPage = 1;
		loadFiles();
	}

	function toggleSortOrder() {
		sortOrder = sortOrder === "asc" ? "desc" : "asc";
		currentPage = 1;
		loadFiles();
	}

	// Breadcrumb segments strictly below the shared root; the root itself is
	// the first (and leftmost) crumb, so navigation can never escape it.
	$: crumbSegments = safeFolderPath
		.split("/")
		.filter(Boolean)
		.map((seg, i, arr) => ({
			label: seg,
			path: arr.slice(0, i + 1).join("/"),
		}));

	onMount(() => {
		if (!$authStore.isAuthenticated) {
			goto("/login");
			return;
		}
		loadRoots();
	});

	// Reload when navigation changes.
	$: if ($authStore.isAuthenticated && driveId && rootPath && safeFolderPath !== undefined) {
		currentPage = 1;
		loadFiles();
	}
</script>

<svelte:head>
	<title>Shared with me — Archivus</title>
</svelte:head>

<div class="min-h-screen bg-gray-50">
	<Navbar />

	<main class="mx-auto max-w-7xl px-4 py-6 sm:px-6 lg:px-8">
		{#if loading}
			<div class="flex items-center justify-center py-24">
				<div
					class="h-8 w-8 animate-spin rounded-full border-4 border-orange-200 border-t-orange-600"
				></div>
			</div>
		{:else if error}
			<div class="rounded-lg bg-red-50 p-4 text-sm text-red-700">{error}</div>
		{:else if !rootPath}
			<!-- Roots: every folder shared with this account -->
			<div class="mb-6">
				<h1 class="text-2xl font-bold text-gray-900">Shared with me</h1>
				<p class="mt-1 text-sm text-gray-500">
					Folders other users have shared with your account.
				</p>
			</div>

			{#if roots.length === 0}
				<div class="flex flex-col items-center justify-center py-24 text-gray-400">
					<Share2 class="mb-3 h-10 w-10" />
					<p class="text-lg font-medium">Nothing shared with you yet</p>
					<p class="text-sm">
						When someone shares a folder with your account it appears here.
					</p>
				</div>
			{:else}
				<div
					class="grid grid-cols-2 gap-4 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6"
				>
					{#each roots as root (root.shareId)}
						<!-- svelte-ignore a11y-no-static-element-interactions -->
						<div
							on:click={() => openRoot(root)}
							on:keydown={(e) => e.key === "Enter" && openRoot(root)}
							role="button"
							tabindex="0"
							class="group flex cursor-pointer flex-col items-center rounded-xl border border-gray-200 bg-white p-4 shadow-sm transition-all duration-150
								hover:-translate-y-0.5 hover:shadow-md"
						>
							<div
								class="mb-3 flex h-20 w-full items-center justify-center overflow-hidden rounded-lg bg-gray-50"
							>
								<Folder class="h-14 w-14 text-orange-500" fill="currentColor" />
							</div>
							<p class="w-full truncate text-center text-sm font-medium text-gray-800" title={root.rootPath}>
								{root.rootPath}
							</p>
							<p class="mt-0.5 w-full truncate text-center text-xs text-gray-400" title={root.driveName}>
								{root.driveName}
							</p>
							<span
								class="mt-2 rounded-full px-2.5 py-0.5 text-xs font-medium
									{root.accessLevel === 'write' ? 'bg-blue-100 text-blue-700' : 'bg-gray-100 text-gray-700'}"
							>
								{root.accessLevel}
							</span>
						</div>
					{/each}
				</div>
			{/if}
		{:else}
			<!-- Browsing inside one shared root -->
			<div class="mb-4 flex items-center gap-3">
				<button
					on:click={backToRoots}
					class="flex items-center gap-1.5 rounded-lg border border-gray-300 bg-white px-3 py-1.5 text-sm font-medium text-gray-600 transition-colors hover:bg-gray-50"
				>
					<ArrowLeft class="h-4 w-4" />
					Shared
				</button>
				<nav class="flex flex-wrap items-center gap-1 text-sm text-gray-600">
					{#each crumbSegments as segment, i}
						{#if i > 0}
							<span class="text-gray-400">/</span>
						{/if}
						<button
							on:click={() => navigate(segment.path)}
							class="max-w-[160px] truncate font-medium transition-colors hover:text-orange-600
								{i === crumbSegments.length - 1 ? 'text-gray-900' : ''}"
							title={segment.label}
						>
							{segment.label}
						</button>
					{/each}
					{#if activeRoot}
						<span class="ml-2 rounded-full bg-gray-100 px-2.5 py-0.5 text-xs font-medium text-gray-600">
							{activeRoot.accessLevel} · from {activeRoot.driveName}
						</span>
					{/if}
				</nav>
			</div>

			<!-- Filter & sort toolbar -->
			<div class="mb-4 flex flex-wrap items-center gap-3">
				<div class="flex flex-wrap items-center gap-1.5">
					<span class="mr-1 flex items-center gap-1 text-sm font-medium text-gray-500">
						<ListFilter class="h-4 w-4" />
						Filter
					</span>
					{#each fileCategories as c}
						<button
							on:click={() => applyFilter(c.value)}
							class="rounded-full border px-3 py-1 text-sm font-medium transition-colors
								{category === c.value
								? 'border-orange-500 bg-orange-500 text-white'
								: 'border-gray-300 bg-white text-gray-600 hover:bg-gray-50'}"
						>
							{c.label}
						</button>
					{/each}
				</div>

				<div class="ml-auto flex items-center gap-2">
					<label for="shared-sort-by" class="text-sm text-gray-500">Sort by</label>
					<select
						id="shared-sort-by"
						value={sortBy}
						on:change={(e) => applySort((e.target as HTMLSelectElement).value as SortBy)}
						class="rounded-lg border border-gray-300 bg-white px-2 py-1.5 text-sm
							focus:border-orange-500 focus:outline-none focus:ring-1 focus:ring-orange-500"
					>
						{#each sortByOptions as opt}
							<option value={opt.value}>{opt.label}</option>
						{/each}
					</select>
					<button
						on:click={toggleSortOrder}
						title={sortOrder === "asc" ? "Ascending" : "Descending"}
						class="flex h-8 w-8 items-center justify-center rounded-lg border border-gray-300
							bg-white text-gray-600 transition-colors hover:bg-gray-50"
					>
						{#if sortOrder === "asc"}
							<ArrowUp class="h-4 w-4" />
						{:else}
							<ArrowDown class="h-4 w-4" />
						{/if}
					</button>
				</div>
			</div>

			<!-- Content -->
			{#if listLoading}
				<div class="flex items-center justify-center py-24">
					<div
						class="h-8 w-8 animate-spin rounded-full border-4 border-orange-200 border-t-orange-600"
					></div>
				</div>
			{:else if listError}
				<div class="rounded-lg bg-red-50 p-4 text-sm text-red-700">{listError}</div>
			{:else if files.length === 0}
				<div class="flex flex-col items-center justify-center py-24 text-gray-400">
					<p class="text-lg font-medium">This shared folder is empty</p>
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
							<FileCard {file} on:download={() => downloadBlob(file)} />
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
						<span class="text-sm text-gray-400">Page {currentPage} of {totalPages} · {total} items</span>
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
		{/if}
	</main>

	<!-- Shared read-only viewer: shared download flow for previews and the CTA -->
	<FileViewerModal
		file={viewerFile}
		open={viewerFile !== null}
		onClose={() => (viewerFile = null)}
		onDownload={downloadBlob}
		fetchBlob={fetchSharedBlob}
	/>
</div>
