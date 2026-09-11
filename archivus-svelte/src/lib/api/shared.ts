import { apiFetch } from '$lib/utils/fetcher';
import { paths, baseUrl, type FileCategory, type SortBy, type SortOrder } from '$lib/data/constants';
import { authStore } from '$lib/stores/auth';
import type { FileMetaData } from '$lib/api/files';

// Folder shares: read-only browsing of folders other users shared with you,
// plus grant/revoke management for drive owners/managers. Drive APIs are
// untouched by this surface.

export type ShareAccessLevel = 'read' | 'write';

export interface SharedRoot {
	shareId: string;
	driveId: string;
	driveName: string;
	driveSlug: string;
	rootPath: string;
	accessLevel: ShareAccessLevel;
	grantedBy: string;
	grantedAt: string;
}

export interface SharedFolderUser {
	userId: string;
	username: string;
	email: string;
	accessLevel: ShareAccessLevel;
	grantedBy: string;
	grantedAt: string;
}

interface SharedRootsResponse {
	roots: SharedRoot[];
}

interface SharedFilesResponse {
	files: FileMetaData[];
	total: number;
	page: number;
	pageSize: number;
}

interface SharedUsersResponse {
	users: SharedFolderUser[];
}

export async function getSharedRoots(): Promise<SharedRoot[]> {
	const data = await apiFetch<SharedRootsResponse>(paths.sharedRoots);
	return data.roots ?? [];
}

// listSharedFiles lists a folder inside a shared subtree. Both rootPath and
// path are drive-relative; path must be the root or inside it.
export async function listSharedFiles(
	driveId: string,
	rootPath: string,
	path: string,
	page = 1,
	pageSize = 24,
	options: { category?: FileCategory | ''; sortBy?: SortBy; sortOrder?: SortOrder } = {}
): Promise<SharedFilesResponse> {
	return apiFetch<SharedFilesResponse>(paths.sharedList, {
		method: 'POST',
		body: JSON.stringify({
			driveId,
			rootPath,
			path,
			page,
			pageSize,
			category: options.category ?? '',
			sortBy: options.sortBy ?? 'name',
			sortOrder: options.sortOrder ?? 'asc'
		})
	});
}

export function downloadSharedFileUrl(fileId: string, driveId: string, rootPath: string): string {
	const params = new URLSearchParams({ fileId, driveId, rootPath });
	return `${baseUrl}${paths.sharedFileDownload}?${params.toString()}`;
}

export async function downloadSharedFile(
	fileId: string,
	driveId: string,
	rootPath: string
): Promise<Blob> {
	const token = authStore.getToken();
	const res = await fetch(downloadSharedFileUrl(fileId, driveId, rootPath), {
		headers: token ? { Authorization: `Bearer ${token}` } : {}
	});
	if (!res.ok) {
		throw new Error(`HTTP ${res.status}: ${res.statusText}`);
	}
	return res.blob();
}

// grantFolderShare shares a folder with an existing user, by userId or
// username (one required). Re-granting updates the access level.
export async function grantFolderShare(
	driveId: string,
	rootPath: string,
	target: { userId?: string; username?: string },
	accessLevel: ShareAccessLevel
): Promise<void> {
	await apiFetch(paths.sharedGrant, {
		method: 'POST',
		body: JSON.stringify({ driveId, rootPath, ...target, accessLevel })
	});
}

export async function revokeFolderShare(
	driveId: string,
	rootPath: string,
	target: { userId?: string; username?: string }
): Promise<void> {
	await apiFetch(paths.sharedRevoke, {
		method: 'POST',
		body: JSON.stringify({ driveId, rootPath, ...target })
	});
}

export async function listFolderShares(
	driveId: string,
	rootPath: string
): Promise<SharedFolderUser[]> {
	const data = await apiFetch<SharedUsersResponse>(paths.sharedListUsers, {
		method: 'POST',
		body: JSON.stringify({ driveId, rootPath })
	});
	return data.users ?? [];
}
