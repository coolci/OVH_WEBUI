const fs = require('fs');
const path = require('path');

const { execFileSync } = require('child_process');
const backendRoutes = JSON.parse(execFileSync('go', ['run', './backend/cmd/routes'], {cwd:path.join(__dirname,'..'),encoding:'utf8'}))
  .map(route => ({...route, path:route.path.replace(/^\/api(?=\/)/, '')}));
if (!backendRoutes.length) throw Error('Backend route registry is empty');

console.log(`Found ${backendRoutes.length} backend routes`);

function scanDir(dir) {
  let results = [];
  const list = fs.readdirSync(dir);
  for (const file of list) {
    const fullPath = path.join(dir, file);
    const stat = fs.statSync(fullPath);
    if (stat.isDirectory()) {
      results = results.concat(scanDir(fullPath));
    } else if (/\.(ts|tsx)$/.test(file)) {
      results.push(fullPath);
    }
  }
  return results;
}

const frontendFiles = scanDir(path.join(__dirname, '../src'));
const apiCalls = [];
// Match strings with api endpoint patterns
const pathRegex = /api\.(get|post|put|delete|patch)\s*(?:<[^;]+?>)?\s*\(\s*(["'`])([^"'`]+)\2/g;


for (const f of frontendFiles) {
  const content = fs.readFileSync(f, 'utf8');
  let match;
  while ((match = pathRegex.exec(content)) !== null) {
    let p = match[3];
    let normalized = p;
    if (normalized.startsWith('/api/')) {
      normalized = normalized.substring(4);
    }
    apiCalls.push({ method: match[1].toUpperCase(), file: path.relative(path.join(__dirname, '..'), f), raw: p, normalized });
  }
}

function matchRoute(calledPath, registeredRoute) {
  // Turn :param into wildcard
  let regPattern = '^' + registeredRoute.replace(/:[a-zA-Z0-9_]+/g, '[^/]+') + '$';
  let reg = new RegExp(regPattern);
  let cleanPath = calledPath.split('?')[0];
  // If called path has template interpolation like ${...}, replace with test string
  cleanPath = cleanPath.replace(/\$\{[^}]+\}/g, 'test_val');
  return reg.test(cleanPath);
}

const missing = [];
for (const call of apiCalls) {
  const found = backendRoutes.some(br => br.method === call.method && matchRoute(call.normalized, br.path));
  if (!found) {
    missing.push(call);
  }
}

const uniqueMissing = {};
for (const mis of missing) {
  if (!uniqueMissing[mis.normalized]) {
    uniqueMissing[mis.normalized] = [];
  }
  uniqueMissing[mis.normalized].push(mis.file);
}

console.log("\nPotential unmatched calls (static templates require review; not a completeness proof):");
console.log(JSON.stringify(uniqueMissing, null, 2));
