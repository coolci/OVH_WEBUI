const fs = require('fs');
const path = require('path');

const mainGo = fs.readFileSync(path.join(__dirname, '../backend/main.go'), 'utf8');
const backendRoutes = [];

// Match api.GET/POST etc, sc.GET/POST etc, vc.GET/POST etc
const routeRegex = /(api|sc|vc)\.(GET|POST|PUT|DELETE|PATCH)\(["']([^"']+)["']/g;
let m;
while ((m = routeRegex.exec(mainGo)) !== null) {
  let prefix = "";
  if (m[1] === "sc") prefix = "/server-control";
  if (m[1] === "vc") prefix = "/vps-control";
  backendRoutes.push({ method: m[2], path: prefix + m[3] });
}

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
const pathRegex = /["'`]((\/api\/|\/server-control\/|\/vps-control\/|\/ovh\/|\/queue\/|\/monitor\/|\/auth\/|\/catalog\/|\/config\/|\/system\/|\/logs\/|\/traffic\/)[a-zA-Z0-9_\-\/:\$]+)["'`]/g;

for (const f of frontendFiles) {
  const content = fs.readFileSync(f, 'utf8');
  let match;
  while ((match = pathRegex.exec(content)) !== null) {
    let p = match[1];
    let normalized = p;
    if (normalized.startsWith('/api/')) {
      normalized = normalized.substring(4);
    }
    apiCalls.push({ file: path.relative(path.join(__dirname, '..'), f), raw: p, normalized });
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
  const found = backendRoutes.some(br => matchRoute(call.normalized, br.path));
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

console.log("\nTrue unmapped backend endpoints in frontend:");
console.log(JSON.stringify(uniqueMissing, null, 2));
