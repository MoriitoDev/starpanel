# Plugins as subprocess + vanilla ESM

Backend plugins run as supervised subprocesses speaking HTTP on localhost (any language, no core recompile); frontend widgets are framework-free ESM rendering into a card with CSS vars. Rejected Go `plugin.so` (fragile ABI + restart) and WASM (harder toolchain) for simplicity, and Svelte-only widgets (forces our toolchain on authors).
