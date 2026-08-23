import { PUBLIC_API_URL } from '$env/static/public';

export const baseUrl = PUBLIC_API_URL ?? 'http://localhost:8080';

export const paths = {
	login: '/auth/login',
	register: '/auth/register',
	userInfo: '/auth/user/info',
	driveInfo: '/auth/drive/info',
	driveUsers: '/auth/drive/users',
	driveAdd: '/auth/drive/add',
	driveRemove: '/auth/drive/remove',
	driveInvite: '/auth/drive/invite',
	files: '/storage/files',
	folderCreate: '/storage/folder/create',
	folderDelete: '/storage/folder/delete',
	fileUpload: '/storage/file/upload',
	fileDownload: '/storage/file/download',
	fileMove: '/storage/file/move',
	fileDelete: '/storage/file/delete',
	recycleBin: '/storage/recyclebin',
	recycleBinRestore: '/storage/recyclebin/restore'
} as const;

// Category filter values accepted by the file listing endpoint. The empty
// string means "no filter" (all files). "others" maps to files whose extension
// is in none of the known categories.
export type FileCategory =
	| 'images'
	| 'videos'
	| 'audio'
	| 'spreadsheets'
	| 'docs'
	| 'code'
	| 'others';

export const fileCategories: { value: FileCategory | ''; label: string }[] = [
	{ value: '', label: 'All' },
	{ value: 'images', label: 'Images' },
	{ value: 'videos', label: 'Videos' },
	{ value: 'audio', label: 'Audio' },
	{ value: 'spreadsheets', label: 'Spreadsheets' },
	{ value: 'docs', label: 'Docs' },
	{ value: 'code', label: 'Code' },
	{ value: 'others', label: 'Others' }
];

export type SortBy = 'name' | 'size' | 'created_at';
export type SortOrder = 'asc' | 'desc';

export const sortByOptions: { value: SortBy; label: string }[] = [
	{ value: 'name', label: 'Name' },
	{ value: 'size', label: 'Size' },
	{ value: 'created_at', label: 'Upload date' }
];
