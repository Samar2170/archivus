import { apiFetch } from '$lib/utils/fetcher';
import { paths } from '$lib/data/constants';

export type ApiKeyAccessLevel = 'read' | 'write';

export interface ApiKeyInfo {
	id: string;
	name: string;
	access_level: ApiKeyAccessLevel;
	created_at: string;
	expires_at: string;
}

// Returned exactly once, at creation time — the plaintext key cannot be
// recovered later, so clients must store it.
export interface CreatedApiKey extends ApiKeyInfo {
	api_key: string;
}

export async function getApiKeys(): Promise<ApiKeyInfo[]> {
	const data = await apiFetch<{ api_keys: ApiKeyInfo[] | null }>(paths.apiKeyList);
	return data.api_keys ?? [];
}

export async function createApiKey(name: string, access: ApiKeyAccessLevel): Promise<CreatedApiKey> {
	return apiFetch<CreatedApiKey>(paths.apiKeyCreate, {
		method: 'POST',
		body: JSON.stringify({ name, access })
	});
}

export async function revokeApiKey(keyId: string): Promise<void> {
	await apiFetch(paths.apiKeyRevoke, {
		method: 'POST',
		body: JSON.stringify({ key_id: keyId })
	});
}
