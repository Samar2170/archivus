<script lang="ts">
	import type { FileMetaData } from "$lib/api/files";
	import { Folder, FileText, File, Film, Image } from "lucide-svelte";

	export let file: FileMetaData;
	export let dragging = false;

	function formatSize(bytes: number): string {
		if (bytes < 1024) return `${bytes} B`;
		if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
		return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
	}

	$: ext = file.Extension?.toLowerCase() ?? "";
	$: isImage = ["jpg", "jpeg", "png", "gif", "webp", "svg"].includes(ext);
	$: isVideo = ["mp4", "mov", "avi", "mkv", "webm"].includes(ext);
	$: isPdf = ext === "pdf";
	$: isDoc = ["doc", "docx"].includes(ext);
	$: isSheet = ["xls", "xlsx", "csv"].includes(ext);
	$: hasThumbnail =
		!!(file.Thumbnail || file.SignedUrl) && (isImage || isVideo);
</script>

<div
	class="group relative flex flex-col items-center rounded-xl border border-gray-200 bg-white p-4
		shadow-sm cursor-pointer select-none transition-all duration-150
		hover:shadow-md hover:-translate-y-0.5
		{dragging ? 'opacity-50 scale-95' : ''}"
>
	<!-- Thumbnail / Icon -->
	<div
		class="mb-3 flex h-20 w-full items-center justify-center overflow-hidden rounded-lg bg-gray-50"
	>
		{#if file.IsDir}
			<Folder class="h-14 w-14 text-gray-600" fill="currentColor" />
		{:else if hasThumbnail}
			<img
				src={file.Thumbnail || file.SignedUrl}
				alt={file.Name}
				class="h-full w-full object-cover rounded-lg"
				loading="lazy"
			/>
		{:else if isPdf}
			<FileText class="h-12 w-12 text-red-400" />
		{:else if isDoc}
			<FileText class="h-12 w-12 text-blue-500" />
		{:else if isSheet}
			<FileText class="h-12 w-12 text-green-500" />
		{:else if isVideo}
			<Film class="h-12 w-12 text-purple-400" />
		{:else if isImage}
			<Image class="h-12 w-12 text-pink-400" />
		{:else}
			<File class="h-12 w-12 text-gray-400" />
		{/if}
	</div>

	<!-- Name -->
	<p
		class="w-full truncate text-center text-sm font-medium text-gray-800"
		title={file.Name}
	>
		{file.Name}
	</p>

	<!-- Size for files -->
	{#if !file.IsDir && file.Size}
		<p class="mt-0.5 text-xs text-gray-400">{formatSize(file.Size)}</p>
	{/if}
</div>
