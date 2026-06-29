import { apiFetch } from '$lib/utils/fetcher';

export interface FolderItem {
	Name: string;
	Path: string;
}

export async function createFolder(folder: string): Promise<void> {
	await apiFetch('folder/add/', {
		method: 'POST',
		body: JSON.stringify({ folder })
	});
}

export async function listFolders(): Promise<FolderItem[]> {
	return apiFetch<FolderItem[]>('folder/list/');
}
