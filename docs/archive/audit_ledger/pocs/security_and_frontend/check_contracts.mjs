import fs from "node:fs";
import path from "node:path";
import { execFileSync } from "node:child_process";
import { createRequire } from "node:module";
import { fileURLToPath } from "node:url";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = process.cwd();
const require = createRequire(path.join(repoRoot, "web", "package.json"));
const ts = require("typescript");

// 1. Export live backend routes via `go run ./cmd/routes`
const rawBackend = execFileSync("go", ["run", "./cmd/routes"], {
  cwd: path.join(repoRoot, "server"),
  encoding: "utf8",
}).replace(/^\uFEFF/, "");
const backendRoutes = JSON.parse(rawBackend);

function normalizePath(p) {
  let clean = p.split("?")[0].trim();
  if (!clean.startsWith("/")) clean = "/" + clean;
  // Ensure /api prefix normalization: keep /health as /health, others if from axios client without /api get /api
  clean = clean.replace(/\/+/g, "/");
  if (clean.length > 1 && clean.endsWith("/")) clean = clean.slice(0, -1);
  // Convert Gin :param or *param and OpenAPI {param} and JS ${...} into {*}
  clean = clean
    .replace(/\/:[A-Za-z0-9_]+/g, "/{*}")
    .replace(/\/\*[A-Za-z0-9_]+/g, "/{**}")
    .replace(/\/\{[^}]+\}/g, "/{*}");
  return clean;
}

const backendMap = new Map(); // "METHOD /norm/path" -> route[]
for (const r of backendRoutes) {
  const key = `${r.method.toUpperCase()} ${normalizePath(r.path)}`;
  if (!backendMap.has(key)) backendMap.set(key, []);
  backendMap.get(key).push(r);
}

// 2. Parse api/openapi.yaml (simple indentation-aware parser for paths & methods)
const openapiText = fs.readFileSync(path.join(repoRoot, "api", "openapi.yaml"), "utf8");
const openapiLines = openapiText.split(/\r?\n/);
const openapiRoutes = [];
let inPaths = false;
let currentPath = null;
const httpMethods = new Set(["get", "post", "put", "delete", "patch", "options", "head"]);

for (let i = 0; i < openapiLines.length; i++) {
  const line = openapiLines[i];
  if (/^paths:\s*$/.test(line)) {
    inPaths = true;
    continue;
  }
  if (inPaths && /^[A-Za-z0-9_-]+:/.test(line)) {
    inPaths = false;
    currentPath = null;
    continue;
  }
  if (!inPaths) continue;
  const pathMatch = line.match(/^  (\/[^:]+):\s*$/);
  if (pathMatch) {
    currentPath = pathMatch[1].trim();
    continue;
  }
  if (currentPath) {
    const methodMatch = line.match(/^    ([a-z]+):\s*$/);
    if (methodMatch && httpMethods.has(methodMatch[1])) {
      openapiRoutes.push({
        method: methodMatch[1].toUpperCase(),
        path: currentPath,
        line: i + 1,
      });
    }
  }
}

// Check OpenAPI server base URL to see if paths in openapi.yaml omit /api or include /api
const openapiHasApiPrefix = openapiRoutes.some((r) => r.path.startsWith("/api/"));

const openapiMap = new Map();
for (const r of openapiRoutes) {
  const fullPath =
    openapiHasApiPrefix || r.path === "/health" ? r.path : `/api${r.path.startsWith("/") ? "" : "/"}${r.path}`;
  const key = `${r.method} ${normalizePath(fullPath)}`;
  if (!openapiMap.has(key)) openapiMap.set(key, []);
  openapiMap.get(key).push({ ...r, fullPath });
}

// 3. Walk src/ and extract all API calls via TypeScript AST
function walkFiles(dir, out = []) {
  for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
    const full = path.join(dir, entry.name);
    if (entry.isDirectory()) walkFiles(full, out);
    else if (/\.(ts|tsx)$/.test(entry.name)) out.push(full);
  }
  return out;
}

function exprToTemplatePath(node, sf) {
  if (!node) return null;
  if (ts.isStringLiteral(node) || ts.isNoSubstitutionTemplateLiteral(node)) {
    return node.text;
  }
  if (ts.isTemplateExpression(node)) {
    let s = node.head.text;
    for (const span of node.templateSpans) {
      // Check if expression is a query string suffix like `${q}` or `?foo=...`
      const exprText = span.expression.getText(sf);
      const prevChar = s[s.length - 1];
      const nextText = span.literal.text;
      if (
        (prevChar !== "/" && (exprText === "q" || exprText === "qs" || exprText.includes("?") || nextText === "")) &&
        !s.endsWith("/")
      ) {
        // e.g. `/api/server-control/${encodeURIComponent(serviceName)}/console${q}`
        s += "";
      } else {
        s += "{param}";
      }
      s += nextText;
    }
    return s;
  }
  if (ts.isBinaryExpression(node) && node.operatorToken.kind === ts.SyntaxKind.PlusToken) {
    const l = exprToTemplatePath(node.left, sf);
    const r = exprToTemplatePath(node.right, sf);
    if (l !== null && r !== null) return l + r;
    if (l !== null) return l + "{param}";
  }
  return null;
}

const feCalls = [];
for (const file of walkFiles(path.join(repoRoot, "web", "src"))) {
  const code = fs.readFileSync(file, "utf8");
  const sf = ts.createSourceFile(file, code, ts.ScriptTarget.Latest, true, file.endsWith(".tsx") ? ts.ScriptKind.TSX : ts.ScriptKind.TS);
  const rel = path.relative(repoRoot, file).replace(/\\/g, "/");

  function visit(node) {
    if (ts.isCallExpression(node)) {
      const { line } = sf.getLineAndCharacterOfPosition(node.getStart(sf));
      // Case A: apiRequest(url, options?)
      if (ts.isIdentifier(node.expression) && node.expression.text === "apiRequest") {
        const rawUrl = exprToTemplatePath(node.arguments[0], sf);
        let method = "GET";
        const optArg = node.arguments[1];
        if (optArg && ts.isObjectLiteralExpression(optArg)) {
          for (const prop of optArg.properties) {
            if (ts.isPropertyAssignment(prop) && prop.name.getText(sf) === "method") {
              const m = exprToTemplatePath(prop.initializer, sf);
              if (m) method = m.toUpperCase();
            }
          }
        }
        if (rawUrl) {
          const full = rawUrl === "/health" || rawUrl.startsWith("/health?") || rawUrl.startsWith("/api/")
            ? rawUrl
            : `/api${rawUrl.startsWith("/") ? "" : "/"}${rawUrl}`;
          feCalls.push({ file: rel, line: line + 1, kind: "apiRequest", method, rawUrl, fullPath: full });
        }
      }
      // Case B: api.get / api.post / api.put / api.delete / api.patch
      if (
        ts.isPropertyAccessExpression(node.expression) &&
        ts.isIdentifier(node.expression.expression) &&
        node.expression.expression.text === "api" &&
        ["get", "post", "put", "delete", "patch"].includes(node.expression.name.text)
      ) {
        const method = node.expression.name.text.toUpperCase();
        const rawUrl = exprToTemplatePath(node.arguments[0], sf);
        if (rawUrl) {
          let full = rawUrl;
          if (full !== "/health" && !full.startsWith("/health?") && !full.startsWith("/api/")) {
            full = `/api${full.startsWith("/") ? "" : "/"}${full}`;
          }
          feCalls.push({ file: rel, line: line + 1, kind: `api.${method.toLowerCase()}`, method, rawUrl, fullPath: full });
        }
      }
      // Case C: fetch(...)
      if (ts.isIdentifier(node.expression) && node.expression.text === "fetch") {
        const rawUrl = exprToTemplatePath(node.arguments[0], sf);
        feCalls.push({ file: rel, line: line + 1, kind: "fetch", method: "FETCH", rawUrl: rawUrl || node.arguments[0]?.getText(sf), fullPath: rawUrl || "" });
      }
    }
    ts.forEachChild(node, visit);
  }
  visit(sf);
}

// Compare FE -> Backend
const feMissingInBackend = [];
for (const c of feCalls) {
  if (c.kind === "fetch") continue;
  const key = `${c.method} ${normalizePath(c.fullPath)}`;
  if (!backendMap.has(key)) {
    feMissingInBackend.push({ ...c, normKey: key });
  }
}

// Compare Backend <-> OpenAPI
const backendMissingInOpenapi = [];
for (const [key, routes] of backendMap.entries()) {
  if (!openapiMap.has(key)) {
    backendMissingInOpenapi.push({ key, routes });
  }
}

const openapiMissingInBackend = [];
for (const [key, routes] of openapiMap.entries()) {
  if (!backendMap.has(key)) {
    openapiMissingInBackend.push({ key, routes });
  }
}

const summary = {
  backendRouteCount: backendRoutes.length,
  backendUniqueMethodPathCount: backendMap.size,
  openapiRouteCount: openapiRoutes.length,
  openapiHasApiPrefix,
  feExtractedCallCount: feCalls.length,
  feFetchCalls: feCalls.filter((c) => c.kind === "fetch"),
  feMissingInBackend,
  backendMissingInOpenapi,
  openapiMissingInBackend,
};

fs.writeFileSync(path.join(__dirname, "contract_diff.json"), JSON.stringify(summary, null, 2));
console.log("Summary:", {
  backendRouteCount: summary.backendRouteCount,
  openapiRouteCount: summary.openapiRouteCount,
  feExtractedCallCount: summary.feExtractedCallCount,
  feFetchCalls: summary.feFetchCalls.length,
  feMissingInBackend: summary.feMissingInBackend.length,
  backendMissingInOpenapi: summary.backendMissingInOpenapi.length,
  openapiMissingInBackend: summary.openapiMissingInBackend.length,
});
console.log("Retraction routes in backend:", backendRoutes.filter((r) => r.path.includes("retraction")));
const bKeys = new Set(backendMissingInOpenapi.map((x) => x.key));
const oKeys = new Set(openapiMissingInBackend.map((x) => x.key));
const aliasOpenapi = new Set(
  [...oKeys].map((k) => k.replace(" /api/servers/", " /api/server-control/").replace(" /api/vps/", " /api/vps-control/"))
);
const aliasBackend = new Set(
  [...bKeys].map((k) => k.replace(" /api/server-control/", " /api/servers/").replace(" /api/vps-control/", " /api/vps/"))
);
console.log(
  "Backend STILL missing in OpenAPI after prefix alias:",
  [...bKeys].filter((k) => !aliasOpenapi.has(k))
);
console.log(
  "OpenAPI STILL missing in Backend after prefix alias:",
  [...oKeys].filter((k) => !aliasBackend.has(k))
);

