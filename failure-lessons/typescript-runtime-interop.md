# TypeScript Runtime Interoperability & Workspace Tooling

This document captures lessons learned regarding TypeScript compilation, CommonJS/ESM interop across different JS runtimes (Bun, Node, Vite), and workspace CLI flag portability.

---

## FL-005: Hybrid CJS/ESM Default Export Resolution Failure in Strict NodeNext

### Context
Compiling and executing TypeScript code using `"module": "NodeNext"` and `"moduleResolution": "NodeNext"` with packages like `ajv` (`ajv/dist/2020`) and `@axe-core/playwright`.

### What happened
When importing hybrid or CommonJS npm packages using standard ESM syntax:
```typescript
import AxeBuilder from "@axe-core/playwright";
import Ajv2020 from "ajv/dist/2020";
```
Executing tests under Bun/Vitest or compiling with `tsc` produced runtime errors:
```text
TypeError: AxeBuilder is not a constructor
TypeError: Ajv2020 is not a constructor
```
Inspecting the imported identifier revealed that the default export was wrapped inside an object `{ default: [Function: ...] }` rather than the constructor function directly.

### Observable symptom
`TypeError: [Class] is not a constructor` at test runtime or during bundler invocation.

### Impact
Broke accessibility testing and client-side JSON Schema validation across different execution environments.

### Incorrect assumption
Assumed that TypeScript's `"esModuleInterop": true` or Bun's module loader would identically synthesize default exports across both pure ESM and legacy CommonJS npm distributions.

### Root cause
**Confirmed.** Different JavaScript runtimes and module resolvers (Node.js ESM, Bun, Vitest, tsx, esbuild) treat CJS module wrappers with subtle variations. Under native ESM in NodeNext, `import pkg from 'cjs-package'` assigns the raw `module.exports` object to `pkg`, which may itself contain a `.default` property created by Babel/Rollup/tsc.

### Why the architecture allowed it
Relying on compiler-synthesized default imports without defensive runtime unwrapping.

### Fix
Standardized on defensive default-export unwrapping for hybrid and CJS packages:
```typescript
// In demo-storefront/tests/e2e.spec.ts
import AxeBuilderPkg from "@axe-core/playwright";
const AxeBuilder = ((AxeBuilderPkg as any).default || AxeBuilderPkg) as any;

// In sdk/src/validator.ts
import Ajv2020Pkg from "ajv/dist/2020.js";
const Ajv2020 = ((Ajv2020Pkg as any).default || Ajv2020Pkg) as any;
```

### Verification
All unit tests in `sdk/tests/client.test.ts` and E2E tests in `demo-storefront/tests/e2e.spec.ts` execute cleanly under both Bun and Node LTS with zero runtime TypeError exceptions.

### Prevention rule
When importing third-party CommonJS or hybrid libraries into TypeScript projects configured with native ESM (`NodeNext`), use defensive default-export unwrapping (`(pkg as any).default || pkg`) to ensure portability across Node.js, Bun, Vitest, and browser bundlers.

### Reusable lesson
Never trust compiler defaults alone when working with dual ESM/CJS npm packages in cross-runtime environments.

### Related code
- `sdk/src/validator.ts`
- `demo-storefront/tests/e2e.spec.ts`

### Status
**Resolved.**

---

## FL-006: Workspace CLI Flag Ordering Incompatibility in Multi-Package Repositories

### Context
Executing build and test commands across multiple workspace packages (`sdk`, `demo-storefront`) using Bun CLI.

### What happened
Running `bun --cwd <dir> run <script>` in Makefiles or scripts failed or executed in the root repository directory depending on whether flags preceded or followed positional arguments.

### Observable symptom
Scripts either failed to find the target `package.json` or ignored the target directory, causing tasks to execute in the wrong working tree context.

### Impact
Inconsistent build execution between local developer machines and CI runners.

### Incorrect assumption
Assumed Bun CLI parses `--cwd` identically to pnpm or yarn, regardless of flag placement.

### Root cause
**Confirmed.** Bun's argument parser handles global flags and subcommand options strictly based on position. Placing `--cwd` after `run` or after script arguments alters how the runner interprets the directory context.

### Why the architecture allowed it
Inconsistent invocation patterns in `Makefile` and automation scripts.

### Fix
Standardized all workspace task invocations on explicit subshell execution (`cd <dir> && bun run <script>`):
```makefile
# In Makefile
typecheck:
	cd sdk && bun run typecheck
	cd demo-storefront && bun run typecheck

test-sdk:
	cd sdk && bun run test

test-e2e: build
	cd demo-storefront && bun run test:e2e
```

### Verification
`make check` runs identically and without failure across macOS (zsh) and Ubuntu Linux (bash in GitHub Actions CI).

### Prevention rule
In multi-package workspaces, prefer POSIX-standard `(cd <dir> && <cmd>)` over toolchain-specific directory flags in Makefiles and CI pipelines.

### Reusable lesson
Explicit POSIX primitives provide superior portability and deterministic behavior compared to toolchain-specific CLI conveniences.

### Related code
- `Makefile`
- `.github/workflows/ci.yml`

### Status
**Resolved.**
