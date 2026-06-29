import { apiFetch, apiUpload } from '$lib/utils/fetcher';

export interface FileMetaData {
	id: string;
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

export interface ListFileMetaData {
	ID: string;
	Name: string;
	IsImage: boolean;
	FilePath: string;
	CreatedAt: string;
	SizeInMb: number;
	UpdatedAt: string;
}

interface FilesResponse {
	files: FileMetaData[];
	size: number;
}

interface ListFilesResponse {
	files: ListFileMetaData[];
}

export async function getFiles(folder: string): Promise<FilesResponse> {
	return apiFetch<FilesResponse>(`files/get/?folder=${encodeURIComponent(folder)}`);
}

export async function listFiles(search: string): Promise<ListFilesResponse> {
	return apiFetch<ListFilesResponse>(`files/list/?search=${encodeURIComponent(search)}`);
}

export async function moveFile(filePath: string, dst: string): Promise<void> {
	await apiFetch('files/move/', {
		method: 'POST',
		body: JSON.stringify({ filePath, dst })
	});
}

export async function uploadFiles(
	files: FileList,
	folder: string,
	onProgress: (percent: number) => void
): Promise<void> {
	const formData = new FormData();
	for (const file of files) {
		formData.append('file', file);
	}
	formData.append('folder', folder);
	await apiUpload('files/upload/', formData, onProgress);
}
