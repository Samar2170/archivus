<script lang="ts">
	import { thumbnailUrl, type FileMetaData } from "$lib/api/files";
	import { Folder } from "lucide-svelte";
	import docIcon from "$assets/doct.webp";
	import sheetIcon from "$assets/excelt.avif";
	import genericIcon from "$assets/file.svg";

	export let file: FileMetaData;
	export let dragging = false;

	function formatSize(megaBytes: number): string {
		if (megaBytes < 1) return `${(megaBytes * 1024).toFixed(2)} KB`;
		return `${megaBytes.toFixed(2)} MB`;
	}

	$: ext = file.Extension?.toLowerCase().replace(/^\./, "") ?? "";
	$: isImage = ["jpg", "jpeg", "png", "gif", "webp", "svg"].includes(ext);
	$: isDoc = ["doc", "docx", "txt", "rtf", "md"].includes(ext);
	$: isSheet = ["xls", "xlsx", "csv"].includes(ext);
	// Prefer a real generated thumbnail (images, videos, PDFs). Fall back to the
	// full-size signed URL only for images that don't have one yet.
	$: thumbSrc = thumbnailUrl(file) || (isImage ? file.SignedUrl : "");
	$: hasThumbnail = !file.IsDir && !!thumbSrc;
	// Static placeholder icon for file types without a backend thumbnail.
	$: iconSrc = isDoc ? docIcon : isSheet ? sheetIcon : genericIcon;
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
				src={thumbSrc}
				alt={file.Name}
				class="h-full w-full object-cover rounded-lg"
				loading="lazy"
			/>
		{:else}
			<img
				src={iconSrc}
				alt={file.Name}
				class="h-14 w-14 object-contain"
				loading="lazy"
			/>
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
