import { apiFetch } from '$lib/utils/fetcher';
import { paths } from '$lib/data/constants';

export async function createFolder(path: string, driveId: string): Promise<void> {
	await apiFetch(paths.folderCreate, {
		method: 'POST',
		body: JSON.stringify({ path, driveId })
	});
}

export async function deleteFolder(path: string, driveId: string): Promise<void> {
	await apiFetch(paths.folderDelete, {
		method: 'POST',
		body: JSON.stringify({ path, driveId })
	});
}
