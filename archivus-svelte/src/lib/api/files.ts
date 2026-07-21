import { apiFetch, apiUpload } from '$lib/utils/fetcher';
import { paths, baseUrl } from '$lib/data/constants';
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
}

export async function getFiles(path: string, driveId: string): Promise<FilesResponse> {
	return apiFetch<FilesResponse>(paths.files, {
		method: 'POST',
		body: JSON.stringify({ path, driveId })
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
