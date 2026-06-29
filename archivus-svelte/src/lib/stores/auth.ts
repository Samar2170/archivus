import { writable } from 'svelte/store';
import { browser } from '$app/environment';

interface AuthState {
	user: string | null;
	token: string | null;
	isAuthenticated: boolean;
}

const STORAGE_KEY = 'auth';

function getInitialState(): AuthState {
	if (!browser) return { user: null, token: null, isAuthenticated: false };
	try {
		const stored = localStorage.getItem(STORAGE_KEY);
		if (stored) {
			const parsed = JSON.parse(stored);
			return {
				user: parsed.user ?? null,
				token: parsed.token ?? null,
				isAuthenticated: !!(parsed.user && parsed.token)
			};
		}
	} catch {
		// ignore
	}
	return { user: null, token: null, isAuthenticated: false };
}

function createAuthStore() {
	const { subscribe, set, update } = writable<AuthState>(getInitialState());

	function persist(state: AuthState) {
		if (browser) {
			localStorage.setItem(STORAGE_KEY, JSON.stringify(state));
		}
	}

	return {
		subscribe,
		setAuth(user: string, token: string) {
			const state: AuthState = { user, token, isAuthenticated: true };
			set(state);
			persist(state);
		},
		signout() {
			const state: AuthState = { user: null, token: null, isAuthenticated: false };
			set(state);
			if (browser) localStorage.removeItem(STORAGE_KEY);
		},
		getToken(): string | null {
			let token: string | null = null;
			const unsubscribe = subscribe((s) => (token = s.token));
			unsubscribe();
			return token;
		}
	};
}

export const authStore = createAuthStore();
