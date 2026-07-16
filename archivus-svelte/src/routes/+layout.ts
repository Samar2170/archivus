// Archivus is a client-side SPA: auth lives in localStorage and every route
// talks to the Go API directly, so there is nothing to render on the server.
export const ssr = false;
export const prerender = false;
