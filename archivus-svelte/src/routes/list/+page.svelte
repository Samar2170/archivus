<script lang="ts">
	import { onMount } from "svelte";
	import { goto } from "$app/navigation";
	import { page } from "$app/stores";
	import { authStore } from "$lib/stores/auth";
	import { listFiles } from "$lib/api/files";
	import type { ListFileMetaData } from "$lib/api/files";
	import Navbar from "$lib/components/Navbar.svelte";
	import { Search, File, Image, ExternalLink } from "lucide-svelte";

	let files: ListFileMetaData[] = [];
	let loading = false;
	let error = "";
	let searchInput = "";

	$: searchParam = $page.url.searchParams.get("search") ?? "";

	async function loadFiles(query: string) {
		loading = true;
		error = "";
		try {
			const result = await listFiles(query);
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
		loadFiles(searchParam);
	});

	$: if ($authStore.isAuthenticated) loadFiles(searchParam);

	function handleSearch(e: Event) {
		e.preventDefault();
		if (searchInput.trim()) {
			goto(`/list?search=${encodeURIComponent(searchInput.trim())}`);
		}
	}

	function formatDate(dateStr: string): string {
		try {
			return new Date(dateStr).toLocaleDateString(undefined, {
				year: "numeric",
				month: "short",
				day: "numeric",
			});
		} catch {
			return dateStr;
		}
	}
</script>

<svelte:head>
	<title>List Files — Archivus</title>
</svelte:head>

<div class="min-h-screen bg-gray-50">
	<Navbar />

	<main class="mx-auto max-w-7xl px-4 py-6 sm:px-6 lg:px-8">
		<div
			class="mb-6 flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between"
		>
			<h1 class="text-xl font-semibold text-gray-900">
				{searchParam ? `Results for "${searchParam}"` : "All Files"}
			</h1>

			<form on:submit={handleSearch} class="flex gap-2">
				<div class="relative">
					<Search
						class="absolute left-2.5 top-1/2 -translate-y-1/2 h-4 w-4 text-gray-400"
					/>
					<input
						type="search"
						bind:value={searchInput}
						placeholder="Search files..."
						class="pl-9 pr-3 py-2 text-sm rounded-lg border border-gray-300
							focus:border-orange-500 focus:outline-none focus:ring-1 focus:ring-orange-500 w-60"
					/>
				</div>
				<button
					type="submit"
					class="rounded-lg bg-orange-600 px-4 py-2 text-sm font-medium text-white hover:bg-orange-700"
				>
					Search
				</button>
			</form>
		</div>

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
				<Search class="h-12 w-12 mb-4 opacity-40" />
				<p class="text-lg font-medium">
					{searchParam ? "No files found" : "No files yet"}
				</p>
			</div>
		{:else}
			<div
				class="overflow-x-auto rounded-xl border border-gray-200 bg-white"
			>
				<table class="min-w-full divide-y divide-gray-200">
					<thead class="bg-gray-50">
						<tr>
							<th
								class="px-4 py-3 text-left text-xs font-semibold uppercase tracking-wide text-gray-500"
								>Name</th
							>
							<th
								class="px-4 py-3 text-left text-xs font-semibold uppercase tracking-wide text-gray-500"
								>Size</th
							>
							<th
								class="px-4 py-3 text-left text-xs font-semibold uppercase tracking-wide text-gray-500"
								>Created</th
							>
							<th
								class="px-4 py-3 text-left text-xs font-semibold uppercase tracking-wide text-gray-500"
								>Updated</th
							>
							<th class="px-4 py-3 w-10"></th>
						</tr>
					</thead>
					<tbody class="divide-y divide-gray-100">
						{#each files as file (file.ID)}
							<tr class="hover:bg-gray-50 transition-colors">
								<td class="px-4 py-3">
									<div class="flex items-center gap-2">
										{#if file.IsImage}
											<Image
												class="h-4 w-4 text-pink-400 flex-shrink-0"
											/>
										{:else}
											<File
												class="h-4 w-4 text-gray-400 flex-shrink-0"
											/>
										{/if}
										<span
											class="text-sm font-medium text-gray-800 truncate max-w-xs"
											title={file.Name}
										>
											{file.Name}
										</span>
									</div>
								</td>
								<td
									class="px-4 py-3 text-sm text-gray-500 whitespace-nowrap"
								>
									{file.SizeInMb?.toFixed(2)} MB
								</td>
								<td
									class="px-4 py-3 text-sm text-gray-500 whitespace-nowrap"
								>
									{formatDate(file.CreatedAt)}
								</td>
								<td
									class="px-4 py-3 text-sm text-gray-500 whitespace-nowrap"
								>
									{formatDate(file.UpdatedAt)}
								</td>
								<td class="px-4 py-3">
									<a
										href={file.FilePath}
										target="_blank"
										rel="noopener noreferrer"
										class="text-gray-400 hover:text-orange-600 transition-colors"
										title="Open file"
									>
										<ExternalLink class="h-4 w-4" />
									</a>
								</td>
							</tr>
						{/each}
					</tbody>
				</table>
			</div>

			<p class="mt-3 text-sm text-gray-400">
				{files.length} file{files.length !== 1 ? "s" : ""}
			</p>
		{/if}
	</main>
</div>
