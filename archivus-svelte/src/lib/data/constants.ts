import { PUBLIC_API_URL } from '$env/static/public';

export const baseUrl = PUBLIC_API_URL ?? 'http://localhost:8080';

export const paths = {
	login: '/auth/login',
	register: '/auth/register',
	userInfo: '/auth/user/info',
	driveInfo: '/auth/drive/info',
	driveUsers: '/auth/drive/users',
	files: '/storage/files',
	folderCreate: '/storage/folder/create',
	folderDelete: '/storage/folder/delete',
	fileUpload: '/storage/file/upload',
	fileDownload: '/storage/file/download'
} as const;
