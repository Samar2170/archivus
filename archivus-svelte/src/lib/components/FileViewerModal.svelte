<script lang="ts">
	import { onDestroy } from "svelte";
	import { X, Download, Loader2, FileWarning } from "lucide-svelte";
	import { downloadFile, type FileMetaData } from "$lib/api/files";
	import { authStore } from "$lib/stores/auth";

	// The file to preview (null = nothing to show) and whether the viewer is
	// visible. The parent owns both; closing is reported via onClose.
	export let file: FileMetaData | null = null;
	export let open = false;
	export let onClose: () => void = () => {};
	// Reuses the parent's blob-download flow for the download CTA.
	export let onDownload: (file: FileMetaData) => void = () => {};
	// Optional alternative fetch for preview bytes. Lets callers that browse
	// outside the drive-membership APIs (e.g. shared folders) supply their own
	// authorized download; when unset the drive download endpoint is used.
	export let fetchBlob: ((file: FileMetaData) => Promise<Blob>) | null = null;
	// Optional inline-URL resolver for object storage. When it returns a URL for
	// a renderable kind, the element streams straight from storage instead of
	// buffering the whole file into a blob. Returning "" falls back to fetchBlob.
	export let resolvePreviewUrl: ((file: FileMetaData) => Promise<string>) | null = null;

	const imageExts = ["jpg", "jpeg", "png", "gif", "webp", "svg"];
	const videoExts = ["mp4", "webm", "mov", "mkv", "avi", "m4v"];
	const textExts = ["txt", "md", "csv", "json"];

	type PreviewKind = "image" | "video" | "pdf" | "text" | "none";

	function previewKind(ext: string): PreviewKind {
		if (imageExts.includes(ext)) return "image";
		if (videoExts.includes(ext)) return "video";
		if (ext === "pdf") return "pdf";
		if (textExts.includes(ext)) return "text";
		return "none";
	}

	// Canonical MIME type per extension. Download endpoints often send
	// application/octet-stream, which browsers refuse to render inline, so the
	// blob is re-typed before creating the preview URL.
	const mimeByExt: Record<string, string> = {
		jpg: "image/jpeg",
		jpeg: "image/jpeg",
		png: "image/png",
		gif: "image/gif",
		webp: "image/webp",
		svg: "image/svg+xml",
		mp4: "video/mp4",
		webm: "video/webm",
		mov: "video/quicktime",
		mkv: "video/x-matroska",
		avi: "video/x-msvideo",
		m4v: "video/x-m4v",
		pdf: "application/pdf",
		txt: "text/plain",
		md: "text/plain",
		csv: "text/plain",
		json: "application/json",
	};

	function withMime(blob: Blob, ext: string): Blob {
		const mime = mimeByExt[ext];
		return mime && blob.type !== mime ? blob.slice(0, blob.size, mime) : blob;
	}

	// Containers like .mov/.mkv/.avi only play when the browser supports their
	// codecs, so probe the browser before fetching bytes we may not decode.
	function canPlayVideo(ext: string): boolean {
		const probe = document.createElement("video");
		return probe.canPlayType(mimeByExt[ext] ?? `video/${ext}`) !== "";
	}

	$: ext = file?.Extension?.toLowerCase().replace(/^\./, "") ?? "";
	$: baseKind = previewKind(ext);
	// HEIC/HEIF and other unknown types fall through to "none": only Safari
	// decodes HEIC natively, so they get the download CTA instead of a broken
	// preview. Unplayable video containers get the same treatment.
	$: kind =
		baseKind === "video" && !canPlayVideo(ext) ? "none" : baseKind;
	$: unsupportedReason =
		baseKind === "video"
			? "This video format isn't supported by your browser."
			: "This file type can't be previewed in the browser.";

	let objectUrl = "";
	let textContent = "";
	let loading = false;
	let error = "";
	// ID of the file whose bytes are currently loaded ("" = none).
	let loadedFileId = "";
	// Bumped on every load/cleanup so stale responses can be discarded.
	let requestToken = 0;

	function releaseObject() {
		if (objectUrl) URL.revokeObjectURL(objectUrl);
		objectUrl = "";
	}

	function cleanup() {
		requestToken++;
		releaseObject();
		textContent = "";
		loading = false;
		error = "";
		loadedFileId = "";
	}

	async function load(target: FileMetaData, targetKind: PreviewKind) {
		const token = ++requestToken;
		releaseObject();
		textContent = "";
		error = "";
		if (!fetchBlob && !$authStore.driveId) {
			error = "No drive available for this account.";
			return;
		}
		loading = true;
		try {
			// Prefer a direct, inline-URL preview for renderable kinds: the
			// browser streams (and can range-seek) instead of buffering the file.
			if (resolvePreviewUrl && targetKind !== "text") {
				const direct = await resolvePreviewUrl(target);
				if (token !== requestToken) return;
				if (direct) {
					objectUrl = direct;
					loadedFileId = target.ID;
					return;
				}
			}
			const blob = fetchBlob ? await fetchBlob(target) : await downloadFile(target.ID, $authStore.driveId!);
			if (token !== requestToken) return;
			if (targetKind === "text") {
				textContent = await blob.text();
				if (token !== requestToken) return;
			} else {
				const targetExt =
					target.Extension?.toLowerCase().replace(/^\./, "") ?? "";
				objectUrl = URL.createObjectURL(withMime(blob, targetExt));
			}
			loadedFileId = target.ID;
		} catch (err) {
			if (token !== requestToken) return;
			error = (err as Error).message;
		} finally {
			if (token === requestToken) loading = false;
		}
	}

	// Load bytes whenever the viewer opens or switches to a different file.
	// Unsupported types skip the fetch entirely (nothing to preview).
	$: if (open && file && file.ID !== loadedFileId) {
		if (kind === "none") {
			loadedFileId = file.ID;
			releaseObject();
			textContent = "";
			error = "";
		} else {
			load(file, kind);
		}
	}

	// Release the object URL as soon as the viewer closes or is cleared.
	$: if ((!open || !file) && loadedFileId) {
		cleanup();
	}

	function onKeydown(e: KeyboardEvent) {
		if (open && e.key === "Escape") onClose();
	}

	onDestroy(cleanup);
</script>

<svelte:window on:keydown={onKeydown} />

{#if open && file}
	<div
		class="fixed inset-0 z-[60] flex items-center justify-center bg-black/40 p-4 backdrop-blur-sm"
	>
		<!-- Backdrop: closes the viewer on any outside click -->
		<button
			class="absolute inset-0 cursor-default"
			aria-label="Close viewer"
			on:click={onClose}
		></button>

		<div
			role="dialog"
			aria-modal="true"
			aria-label={file.Name}
			class="relative flex max-h-[90vh] w-full max-w-4xl flex-col overflow-hidden rounded-2xl bg-white shadow-xl"
		>
			<!-- Header -->
			<div class="flex items-center justify-between gap-3 border-b border-gray-100 px-4 py-3">
				<p class="truncate text-sm font-semibold text-gray-800" title={file.Name}>
					{file.Name}
				</p>
				<button
					on:click={onClose}
					aria-label="Close viewer"
					class="shrink-0 rounded-lg p-1.5 text-gray-400 transition-colors hover:bg-gray-100 hover:text-gray-600"
				>
					<X class="h-5 w-5" />
				</button>
			</div>

			<!-- Body -->
			<div class="flex min-h-0 flex-1 items-center justify-center overflow-auto p-4">
				{#if loading}
					<div class="flex flex-col items-center gap-3 py-16 text-gray-400">
						<Loader2 class="h-8 w-8 animate-spin" />
						<p class="text-sm">Loading preview…</p>
					</div>
				{:else if error}
					<div class="flex flex-col items-center gap-3 py-16 text-center">
						<FileWarning class="h-10 w-10 text-red-400" />
						<p class="text-sm text-red-600">Couldn't load this file.</p>
						<p class="max-w-sm text-xs text-gray-500">{error}</p>
						<button
							on:click={() => load(file, kind)}
							class="rounded-lg border border-gray-300 px-4 py-2 text-sm font-medium text-gray-700 hover:bg-gray-50"
						>
							Try again
						</button>
					</div>
				{:else if kind === "image"}
					<img
						src={objectUrl}
						alt={file.Name}
						class="max-h-[72vh] w-auto max-w-full rounded-lg object-contain"
					/>
				{:else if kind === "video"}
					<!-- svelte-ignore a11y_media_has_caption -->
					<video
						src={objectUrl}
						controls
						playsinline
						class="max-h-[72vh] w-full rounded-lg"
					></video>
				{:else if kind === "pdf"}
					<iframe
						src={objectUrl}
						title={file.Name}
						class="h-[75vh] w-full rounded-lg border border-gray-200"
					></iframe>
				{:else if kind === "text"}
					<pre
						class="max-h-[72vh] w-full overflow-auto whitespace-pre-wrap break-words rounded-lg bg-gray-50 p-4 text-sm leading-relaxed text-gray-800">{textContent}</pre>
				{:else}
					<div class="flex flex-col items-center gap-3 py-16 text-center">
						<FileWarning class="h-10 w-10 text-gray-400" />
						<p class="text-sm font-medium text-gray-700">
							Preview not available
						</p>
						<p class="text-xs text-gray-500">{unsupportedReason}</p>
						<button
							on:click={() => onDownload(file)}
							class="mt-1 flex items-center gap-2 rounded-lg bg-orange-600 px-4 py-2 text-sm font-medium text-white hover:bg-orange-700"
						>
							<Download class="h-4 w-4" />
							Download file
						</button>
					</div>
				{/if}
			</div>
		</div>
	</div>
{/if}
