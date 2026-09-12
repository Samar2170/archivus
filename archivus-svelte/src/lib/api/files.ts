import { apiFetch, apiUpload } from '$lib/utils/fetcher';
import { paths, baseUrl, type FileCategory, type SortBy, type SortOrder } from '$lib/data/constants';
import { authStore } from '$lib/stores/auth';

export interface FileMetaData {
	ID: string;
	Name: string;
	IsDir: boolean;
	Extension: string;
	SignedUrl: string;
	Size: number;
	Path: string;
	NavigationPath: string;
	Thumbnail: string;
}

interface FilesResponse {
	files: FileMetaData[];
	total: number;
	page: number;
	pageSize: number;
}

export interface ListFilesOptions {
	category?: FileCategory | '';
	sortBy?: SortBy;
	sortOrder?: SortOrder;
}

export async function getFiles(
	path: string,
	driveId: string,
	page = 1,
	pageSize = 24,
	options: ListFilesOptions = {}
): Promise<FilesResponse> {
	return apiFetch<FilesResponse>(paths.files, {
		method: 'POST',
		body: JSON.stringify({
			path,
			driveId,
			page,
			pageSize,
			category: options.category ?? '',
			sortBy: options.sortBy ?? 'name',
			sortOrder: options.sortOrder ?? 'asc'
		})
	});
}

export async function uploadFiles(
	files: FileList,
	folderPath: string,
	driveId: string,
	onProgress: (percent: number) => void
): Promise<void> {
	const formData = new FormData();
	for (const file of files) {
		formData.append('files', file);
	}
	formData.append('folderPath', folderPath);
	formData.append('driveId', driveId);
	await apiUpload(paths.fileUpload, formData, onProgress);
}

// thumbnailUrl resolves a file's server-relative thumbnail path (e.g.
// "/storage/thumbnails/…") into an absolute URL. Returns "" when the file has
// no generated thumbnail.
export function thumbnailUrl(file: FileMetaData): string {
	return file.Thumbnail ? `${baseUrl}${file.Thumbnail}` : '';
}

export function downloadFileUrl(fileId: string, driveId: string): string {
	const params = new URLSearchParams({ fileId, driveId });
	return `${baseUrl}${paths.fileDownload}?${params.toString()}`;
}

// moveFile relocates a file into dstPath (a folder relative to the drive root;
// empty string means the drive root), keeping its name.
export async function moveFile(
	fileId: string,
	dstPath: string,
	driveId: string
): Promise<void> {
	await apiFetch(paths.fileMove, {
		method: 'POST',
		body: JSON.stringify({ fileId, dstPath, driveId })
	});
}

// deleteFile moves a file into the recycle bin, where it is kept for 30 days
// before being permanently purged.
export async function deleteFile(fileId: string, driveId: string): Promise<void> {
	await apiFetch(paths.fileDelete, {
		method: 'POST',
		body: JSON.stringify({ fileId, driveId })
	});
}

export interface RecycleEntry {
	ID: string;
	Name: string;
	IsDir: boolean;
	Size: number;
	ContentType: string;
	OriginalPath: string;
	DeletedAt: string; // RFC3339
	ExpiresAt: string; // RFC3339
}

interface RecycleBinResponse {
	items: RecycleEntry[];
}

// getRecycleBin lists a drive's deleted files awaiting permanent purge.
export async function getRecycleBin(driveId: string): Promise<RecycleBinResponse> {
	return apiFetch<RecycleBinResponse>(paths.recycleBin, {
		method: 'POST',
		body: JSON.stringify({ driveId })
	});
}

// restoreFile moves a recycle bin item back to its original location.
export async function restoreFile(recycleBinId: string, driveId: string): Promise<void> {
	await apiFetch(paths.recycleBinRestore, {
		method: 'POST',
		body: JSON.stringify({ recycleBinId, driveId })
	});
}

// purgeRecycleBinItem permanently deletes a single recycle bin item (file or
// folder) immediately, skipping the remaining 30 day retention window. This
// cannot be undone.
export async function purgeRecycleBinItem(recycleBinId: string, driveId: string): Promise<void> {
	await apiFetch(paths.recycleBinPurge, {
		method: 'POST',
		body: JSON.stringify({ recycleBinId, driveId })
	});
}

export async function downloadFile(fileId: string, driveId: string): Promise<Blob> {
	const token = authStore.getToken();
	const res = await fetch(downloadFileUrl(fileId, driveId), {
		headers: token ? { Authorization: `Bearer ${token}` } : {}
	});
	if (!res.ok) {
		throw new Error(`HTTP ${res.status}: ${res.statusText}`);
	}
	return res.blob();
}

// requestDownloadUrl asks the server for a short-lived direct-download URL.
// Object-storage backends return one so the browser can fetch bytes straight
// from the bucket; local-disk backends return "" and the caller must stream.
export async function requestDownloadUrl(fileId: string, driveId: string): Promise<string> {
	const params = new URLSearchParams({ fileId, driveId });
	const data = await apiFetch<{ url: string }>(`${paths.fileDownloadUrl}?${params.toString()}`);
	return data.url ?? '';
}

// requestPreviewUrl asks for an inline URL usable directly as an element src
// (image/video/pdf), so large previews stream instead of buffering a blob.
export async function requestPreviewUrl(fileId: string, driveId: string): Promise<string> {
	const params = new URLSearchParams({ fileId, driveId, mode: 'inline' });
	const data = await apiFetch<{ url: string }>(`${paths.fileDownloadUrl}?${params.toString()}`);
	return data.url ?? '';
}

// triggerBrowserDownload hands a URL to the browser and lets it stream the
// bytes itself, so the whole file never sits in JS memory. The cross-origin
// `download` attribute is ignored by browsers, so the server-side
// Content-Disposition (set by the presigned URL) supplies the filename.
export function triggerBrowserDownload(url: string, filename: string): void {
	const a = document.createElement('a');
	a.href = url;
	a.download = filename;
	a.rel = 'noopener';
	document.body.appendChild(a);
	a.click();
	a.remove();
}

// downloadFileToDisk downloads a file, preferring a direct object-storage URL
// and falling back to the authenticated stream when the backend cannot mint
// one (local disk).
export async function downloadFileToDisk(file: FileMetaData, driveId: string): Promise<void> {
	let url = '';
	try {
		url = await requestDownloadUrl(file.ID, driveId);
	} catch {
		// Minting failed; the fallback below will surface any real error.
	}
	if (url) {
		triggerBrowserDownload(url, file.Name);
		return;
	}
	const blob = await downloadFile(file.ID, driveId);
	const objectUrl = URL.createObjectURL(blob);
	triggerBrowserDownload(objectUrl, file.Name);
	URL.revokeObjectURL(objectUrl);
}
