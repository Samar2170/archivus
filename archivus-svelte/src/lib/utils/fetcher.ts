import { authStore } from '$lib/stores/auth';
import { baseUrl } from '$lib/data/constants';

export async function apiFetch<T>(path: string, options: RequestInit = {}): Promise<T> {
	const token = authStore.getToken();

	const headers: Record<string, string> = {};

	if (token) {
		headers['Authorization'] = `Bearer ${token}`;
	}

	if (!(options.body instanceof FormData)) {
		headers['Content-Type'] = 'application/json';
	}

	const response = await fetch(`${baseUrl}${path}`, {
		...options,
		headers: {
			...headers,
			...(options.headers as Record<string, string>)
		}
	});

	if (!response.ok) {
		throw new Error(`HTTP ${response.status}: ${response.statusText}`);
	}

	return response.json() as Promise<T>;
}

export function apiUpload(
	path: string,
	body: FormData,
	onProgress: (percent: number) => void
): Promise<unknown> {
	return new Promise((resolve, reject) => {
		const token = authStore.getToken();
		const xhr = new XMLHttpRequest();
		xhr.open('POST', `${baseUrl}${path}`);
		if (token) xhr.setRequestHeader('Authorization', `Bearer ${token}`);

		xhr.upload.addEventListener('progress', (e) => {
			if (e.lengthComputable) onProgress(Math.round((e.loaded / e.total) * 100));
		});

		xhr.addEventListener('load', () => {
			if (xhr.status >= 200 && xhr.status < 300) {
				resolve(JSON.parse(xhr.responseText));
			} else {
				reject(new Error(`HTTP ${xhr.status}`));
			}
		});

		xhr.addEventListener('error', () => reject(new Error('Network error')));
		xhr.send(body);
	});
}
