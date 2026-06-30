import { apiFetch } from '$lib/utils/fetcher';
import { paths } from '$lib/data/constants';
import { authStore } from '$lib/stores/auth';

interface LoginResponse {
	token: string;
}

export interface User {
	ID: string;
	Username: string;
	Email: string;
	IsAdmin: boolean;
	Type: string;
}

export interface DriveUser {
	UserID: string;
	DriveID: string;
	DriveName: string;
	AccessLevel: string;
}

export interface UserInfoResponse {
	user: User;
	drives: DriveUser[];
}

export async function getUserInfo(): Promise<UserInfoResponse> {
	return apiFetch<UserInfoResponse>(paths.userInfo);
}

export async function signin(username: string, password: string, pin: string): Promise<void> {
	const data = await apiFetch<LoginResponse>(paths.login, {
		method: 'POST',
		body: JSON.stringify({ username, password, pin })
	});
	authStore.setAuth(username, data.token);

	// The new backend scopes all storage operations to a drive, so resolve the
	// user's active drive immediately after login and cache its id.
	try {
		const info = await getUserInfo();
		const driveId = info.drives?.[0]?.DriveID ?? null;
		authStore.setDriveId(driveId);
	} catch {
		authStore.setDriveId(null);
	}
}
