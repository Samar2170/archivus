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

export function downloadFileUrl(fileId: string, driveId: string): string {
	const params = new URLSearchParams({ fileId, driveId });
	return `${baseUrl}${paths.fileDownload}?${params.toString()}`;
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
