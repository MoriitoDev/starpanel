import { defineConfig } from "vite";
import { svelte } from "@sveltejs/vite-plugin-svelte";
import tailwindcss from "@tailwindcss/vite";

export default defineConfig({
  plugins: [tailwindcss(), svelte()],
  server: {
    port: 5173,
    proxy: { "/api/v1": "http://localhost:8080" }
  },
  // The build lands inside the Go package because //go:embed cannot reach
  // outside its own directory. Everything else about the app is unchanged.
  build: { outDir: "../api/webdist", emptyOutDir: true }
});
