import { apiFetch } from '$lib/utils/fetcher';
import { paths } from '$lib/data/constants';
import { authStore } from '$lib/stores/auth';

interface LoginResponse {
	token: string;
}

export async function signin(username: string, password: string, pin: string): Promise<void> {
	const data = await apiFetch<LoginResponse>(paths.login, {
		method: 'POST',
		body: JSON.stringify({ username, password, pin })
	});
	authStore.setAuth(username, data.token);
}
