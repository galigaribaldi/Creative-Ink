#!/usr/bin/env node
// Compiles every .svelte component and .svelte.js rune module of the Creative-ink frontend with the project's
// own Svelte compiler and reports errors and warnings. Read-only: it writes
// nothing and installs nothing.
//
// Usage (from the repo root):
//   node .claude/skills/ui-svelte-architect/scripts/check-svelte.mjs [frontendDir]
import { createRequire } from 'node:module'
import { readdirSync, readFileSync, existsSync } from 'node:fs'
import { dirname, join, relative, resolve } from 'node:path'
import { fileURLToPath, pathToFileURL } from 'node:url'

const here = dirname(fileURLToPath(import.meta.url))
const repoRoot = resolve(here, '../../../..')
const frontend = resolve(process.argv[2] ?? join(repoRoot, 'creative-ink/frontend'))
const srcDir = join(frontend, 'src')

if (!existsSync(join(frontend, 'node_modules/svelte'))) {
  console.error(`No se encontró svelte en ${frontend}/node_modules. Pide al programador: cd creative-ink/frontend && npm install`)
  process.exit(2)
}

// Resolve the compiler from the frontend, not from this script's location.
const requireFromFrontend = createRequire(join(frontend, 'package.json'))
const compilerModule = await import(pathToFileURL(requireFromFrontend.resolve('svelte/compiler')).href)
// The "require" export is CommonJS: its API lands on `default` when imported.
const { compile, compileModule, VERSION } = compilerModule.compile ? compilerModule : compilerModule.default

function* svelteFiles(dir) {
  for (const entry of readdirSync(dir, { withFileTypes: true })) {
    const path = join(dir, entry.name)
    if (entry.isDirectory()) yield* svelteFiles(path)
    // .svelte components and .svelte.js / .svelte.ts rune modules.
    else if (/\.svelte(\.[jt]s)?$/.test(entry.name)) yield path
  }
}

let errors = 0
let warnings = 0
for (const file of svelteFiles(srcDir)) {
  const name = relative(frontend, file)
  try {
    const source = readFileSync(file, 'utf8')
    const { warnings: list } = file.endsWith('.svelte')
      ? compile(source, { filename: name, generate: 'client' })
      : compileModule(source, { filename: name, generate: 'client' })
    if (list.length === 0) {
      console.log(`✓ ${name}`)
      continue
    }
    for (const w of list) {
      warnings++
      console.log(`⚠ ${name}:${w.start?.line ?? '?'} [${w.code}] ${w.message.split('\n')[0]}`)
    }
  } catch (err) {
    errors++
    console.log(`✗ ${name}:${err.start?.line ?? '?'} [${err.code ?? 'error'}] ${String(err.message).split('\n')[0]}`)
  }
}

console.log(`\nSvelte ${VERSION}: ${errors} error(es), ${warnings} advertencia(s)`)
process.exit(errors > 0 ? 1 : 0)
