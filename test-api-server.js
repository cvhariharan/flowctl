#!/usr/bin/env node

/**
 * Mock API Server for testing remote options functionality
 * 
 * Provides several endpoints that return options based on query parameters
 * 
 * Usage: node test-api-server.js
 * Then access endpoints like:
 *   - http://localhost:3000/api/cities?country=United%20States
 *   - http://localhost:3000/api/regions?country=Canada&city=Toronto
 *   - http://localhost:3000/api/services?env=production
 */

const http = require('http');
const url = require('url');

// Mock data
const data = {
  cities: {
    'United States': [
      { name: 'New York' },
      { name: 'Los Angeles' },
      { name: 'Chicago' },
      { name: 'Houston' },
      { name: 'Phoenix' }
    ],
    'Canada': [
      { name: 'Toronto' },
      { name: 'Vancouver' },
      { name: 'Montreal' },
      { name: 'Calgary' }
    ],
    'Mexico': [
      { name: 'Mexico City' },
      { name: 'Guadalajara' },
      { name: 'Cancún' }
    ]
  },
  regions: {
    'New York:United States': [
      { name: 'us-east-1' },
      { name: 'us-east-2' }
    ],
    'Los Angeles:United States': [
      { name: 'us-west-1' },
      { name: 'us-west-2' }
    ],
    'Toronto:Canada': [
      { name: 'ca-central-1' }
    ],
    'Vancouver:Canada': [
      { name: 'ca-west-1' }
    ],
    'Mexico City:Mexico': [
      { name: 'mx-central-1' }
    ]
  },
  services: {
    'development': [
      { name: 'api-dev', selected: false },
      { name: 'web-dev', selected: false },
      { name: 'db-dev', selected: false }
    ],
    'staging': [
      { name: 'api-staging', selected: false },
      { name: 'web-staging', selected: false },
      { name: 'db-staging', selected: false }
    ],
    'production': [
      { name: 'api-prod', selected: true },
      { name: 'web-prod', selected: false },
      { name: 'db-prod', selected: false }
    ]
  },
  environments: [
    { name: 'development' },
    { name: 'staging' },
    { name: 'production' }
  ]
};

const server = http.createServer((req, res) => {
  // Enable CORS
  res.setHeader('Access-Control-Allow-Origin', '*');
  res.setHeader('Access-Control-Allow-Methods', 'GET, OPTIONS');
  res.setHeader('Access-Control-Allow-Headers', 'Content-Type');
  res.setHeader('Content-Type', 'application/json');

  if (req.method === 'OPTIONS') {
    res.writeHead(200);
    res.end();
    return;
  }

  const parsedUrl = url.parse(req.url, true);
  const pathname = parsedUrl.pathname;
  const query = parsedUrl.query;

  console.log(`[${new Date().toISOString()}] ${req.method} ${req.url}`);

  // Route: /api/environments
  if (pathname === '/api/environments') {
    res.writeHead(200);
    res.end(JSON.stringify(data.environments));
    return;
  }

  // Route: /api/cities
  if (pathname === '/api/cities') {
    const country = query.country || '';
    const cities = data.cities[country] || [];
    res.writeHead(200);
    res.end(JSON.stringify(cities));
    return;
  }

  // Route: /api/regions
  if (pathname === '/api/regions') {
    const city = query.city || '';
    const country = query.country || '';
    const key = `${city}:${country}`;
    const regions = data.regions[key] || [];
    res.writeHead(200);
    res.end(JSON.stringify(regions));
    return;
  }

  // Route: /api/services
  if (pathname === '/api/services') {
    const env = query.env || 'development';
    const services = data.services[env] || [];
    res.writeHead(200);
    res.end(JSON.stringify(services));
    return;
  }

  // Health check
  if (pathname === '/health') {
    res.writeHead(200);
    res.end(JSON.stringify({ status: 'ok' }));
    return;
  }

  // 404
  res.writeHead(404);
  res.end(JSON.stringify({ error: 'Not found' }));
});

const PORT = process.env.PORT || 3000;
server.listen(PORT, () => {
  console.log(`\n📡 Mock API Server listening on http://localhost:${PORT}`);
  console.log('\n📍 Available endpoints:\n');
  console.log('  GET /api/environments');
  console.log('    Returns: development, staging, production\n');
  console.log('  GET /api/cities?country=<country>');
  console.log('    Example: http://localhost:3000/api/cities?country=United%20States');
  console.log('    Returns: Cities in the specified country\n');
  console.log('  GET /api/regions?city=<city>&country=<country>');
  console.log('    Example: http://localhost:3000/api/regions?city=New%20York&country=United%20States');
  console.log('    Returns: Regions for the specified city and country\n');
  console.log('  GET /api/services?env=<environment>');
  console.log('    Example: http://localhost:3000/api/services?env=production');
  console.log('    Returns: Services available in the specified environment\n');
  console.log('  GET /health');
  console.log('    Returns: health check status\n');
  console.log('🧪 Test with curl:\n');
  console.log('  curl "http://localhost:3000/api/cities?country=Canada"');
  console.log('  curl "http://localhost:3000/api/services?env=production"\n');
  console.log('Press Ctrl+C to stop the server\n');
});

// Graceful shutdown
process.on('SIGINT', () => {
  console.log('\n\nShutting down server...');
  server.close(() => {
    console.log('Server closed');
    process.exit(0);
  });
});
