import { PUBLIC_API_URL } from '$env/static/public';

export const baseUrl = PUBLIC_API_URL ?? 'http://localhost:8000/';

export const paths = {
	login: 'login'
} as const;
