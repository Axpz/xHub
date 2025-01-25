import type { NextConfig } from 'next';

const nextConfig: NextConfig = {
  experimental: {
    ppr: false,
  },
};

module.exports = {
  output: 'standalone',
};

export default nextConfig;
