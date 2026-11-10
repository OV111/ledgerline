import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

export default defineConfig({
  plugins: [react()],
  server: {
    // Ledgerline's own port (see .env.example).
    port: 5180,
    // If 5180 is busy, stop with an error instead of silently using
    // another port.
    strictPort: true,
  },
});
