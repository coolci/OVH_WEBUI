const { execSync } = require('child_process');
const fs = require('fs');
const path = require('path');

const [,, upstreamRel, localRel] = process.argv;
if (!upstreamRel || !localRel) {
  console.error("Usage: node sync_upstream.cjs <upstreamRel> <localRel>");
  process.exit(1);
}

const raw = execSync(`git show upstream/main:${upstreamRel}`, { maxBuffer: 20 * 1024 * 1024 }).toString('utf-8');
const replaced = raw.replace(/github\.com\/ovh-buy\/server/g, 'github.com/ovh-webui/server');
const target = path.resolve(process.cwd(), localRel);
fs.mkdirSync(path.dirname(target), { recursive: true });
fs.writeFileSync(target, replaced, 'utf-8');
console.log(`Synced upstream/main:${upstreamRel} -> ${localRel} (${replaced.length} chars)`);
