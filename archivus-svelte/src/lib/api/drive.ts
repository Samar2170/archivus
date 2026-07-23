import { apiFetch } from '$lib/utils/fetcher';
import { paths } from '$lib/data/constants';
import type { User } from '$lib/api/auth';

export type AccessLevel = 'read' | 'write' | 'manager';

export interface Drive {
	ID: string;
	Name: string;
	Slug: string;
	OwnerID: string;
}

export interface DriveInfoResponse {
	id: string;
	name: string;
	slug: string;
	ownerId: string;
	ownerName: string;
	readUsers: User[] | null;
	writeUsers: User[] | null;
	managerUsers: User[] | null;
}

export interface UsersInDrive {
	drive: Drive;
	readUsers: User[] | null;
	writeUsers: User[] | null;
	managerUsers: User[] | null;
}

interface DriveUsersResponse {
	users: UsersInDrive[] | null;
}

export async function getDriveInfo(driveId: string): Promise<DriveInfoResponse> {
	const params = new URLSearchParams({ drive_id: driveId });
	return apiFetch<DriveInfoResponse>(`${paths.driveInfo}?${params}`);
}

export async function getDriveUsers(): Promise<UsersInDrive[]> {
	const data = await apiFetch<DriveUsersResponse>(paths.driveUsers);
	return data.users ?? [];
}

export async function inviteUser(driveId: string, access: AccessLevel): Promise<string> {
	const data = await apiFetch<{ invite_code: string }>(paths.driveInvite, {
		method: 'POST',
		body: JSON.stringify({ drive_id: driveId, access })
	});
	return data.invite_code;
}

export async function addUserToDrive(
	userId: string,
	driveId: string,
	accessLevel: AccessLevel
): Promise<void> {
	await apiFetch(paths.driveAdd, {
		method: 'POST',
		body: JSON.stringify({ user_id: userId, drive_id: driveId, access_level: accessLevel })
	});
}

export async function removeUserFromDrive(userId: string, driveId: string): Promise<void> {
	await apiFetch(paths.driveRemove, {
		method: 'POST',
		body: JSON.stringify({ user_id: userId, drive_id: driveId })
	});
}
