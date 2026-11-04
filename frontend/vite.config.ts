import { defineConfig, loadEnv } from "vite";
import react from "@vitejs/plugin-react";

// Server-side only: the internal credential is never prefixed VITE_ or bundled.
export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd(), "");
  const workspace = env.AUTOMATION_WORKSPACE_ID || "northstar";
  if (!/^[a-zA-Z0-9_-]+$/.test(workspace))
    throw new Error("Invalid workspace ID");
  return {
    plugins: [
      react(),
      {
        name: "workspace-api-boundary",
        configureServer(server) {
          server.middlewares.use((req, res, next) => {
            if (
              req.url?.startsWith("/api") &&
              !/^\/api\/(catalog|automations|templates)(\/[a-zA-Z0-9_-]+)?(\/(publish|disable))?(\?.*)?$/.test(
                req.url,
              )
            ) {
              res.statusCode = 404;
              res.end("Not found");
              return;
            }
            next();
          });
        },
      },
    ],
    server: {
      proxy: {
        "/api/": {
          target: env.AUTOMATION_API_URL || "http://127.0.0.1:8080",
          headers: {
            Authorization: `Bearer ${env.AUTOMATION_API_TOKEN || "local-development-internal-token"}`,
          },
          rewrite: (path) =>
            path.replace(/^\/api\//, `/v1/workspaces/${workspace}/`),
        },
      },
    },
  };
});
