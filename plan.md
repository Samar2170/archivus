Update click behavior in archivus-svelte/src/routes/+page.svelte:
- Keep folder navigation as-is.
- Replace file window.open(...) flow with openViewer(file) that opens a modal.
- Keep context-menu “Open” wired to the same viewer flow.
Add a reusable viewer modal component, e.g. archivus-svelte/src/lib/components/FileViewerModal.svelte:
- Props: file, open, and close handler.
- On open, fetch file bytes via downloadFile(file.ID, driveId) and create object URL.
- Render by extension:
- Images: jpg/jpeg/png/gif/webp/svg via <img>
- Videos: mp4/webm/mov/mkv/avi/m4v via <video controls>
- Docs preview: pdf via <iframe>, txt/md/csv/json via text decode + <pre>
- Unsupported (doc/docx/xls/xlsx/ppt/pptx): “Preview not available” + download CTA
- Include loading/error states and proper object URL cleanup on close/switch.
Add explicit download icon to each file card in archivus-svelte/src/lib/components/FileCard.svelte:
- Show only for non-folder files.
- Emit a download event (or callback) from icon click.
- Stop propagation so clicking icon does not open viewer.
Wire download icon handling in archivus-svelte/src/routes/+page.svelte:
- Reuse existing blob download logic (handleMenuDownload) for consistent behavior.
- Keep right-click menu download unchanged.
Verify UX + behavior:
- File card click opens viewer first (no immediate browser download).
- Download only occurs from card download icon or context-menu download.
- Viewer works on desktop/mobile and closes cleanly without URL leaks.