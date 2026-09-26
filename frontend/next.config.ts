import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  // Build mandiri untuk image Docker yang ramping (.next/standalone/server.js).
  output: "standalone",
};

export default nextConfig;
