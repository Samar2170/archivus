import { writable } from 'svelte/store';
import { browser } from '$app/environment';

interface AuthState {
	user: string | null;
	token: string | null;
	driveId: string | null;
	isAuthenticated: boolean;
}

const STORAGE_KEY = 'auth';

function getInitialState(): AuthState {
	if (!browser) return { user: null, token: null, driveId: null, isAuthenticated: false };
	try {
		const stored = localStorage.getItem(STORAGE_KEY);
		if (stored) {
			const parsed = JSON.parse(stored);
			return {
				user: parsed.user ?? null,
				token: parsed.token ?? null,
				driveId: parsed.driveId ?? null,
				isAuthenticated: !!(parsed.user && parsed.token)
			};
		}
	} catch {
		// ignore
	}
	return { user: null, token: null, driveId: null, isAuthenticated: false };
}

function createAuthStore() {
	const { subscribe, set, update } = writable<AuthState>(getInitialState());

	function persist(state: AuthState) {
		if (browser) {
			localStorage.setItem(STORAGE_KEY, JSON.stringify(state));
		}
	}

	function read<T>(selector: (s: AuthState) => T): T {
		let value!: T;
		const unsubscribe = subscribe((s) => (value = selector(s)));
		unsubscribe();
		return value;
	}

	return {
		subscribe,
		setAuth(user: string, token: string) {
			update((s) => {
				const state: AuthState = { ...s, user, token, isAuthenticated: true };
				persist(state);
				return state;
			});
		},
		setDriveId(driveId: string | null) {
			update((s) => {
				const state: AuthState = { ...s, driveId };
				persist(state);
				return state;
			});
		},
		signout() {
			const state: AuthState = {
				user: null,
				token: null,
				driveId: null,
				isAuthenticated: false
			};
			set(state);
			if (browser) localStorage.removeItem(STORAGE_KEY);
		},
		getToken(): string | null {
			return read((s) => s.token);
		},
		getDriveId(): string | null {
			return read((s) => s.driveId);
		}
	};
}

export const authStore = createAuthStore();
