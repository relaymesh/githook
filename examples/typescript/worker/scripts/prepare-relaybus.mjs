import { existsSync, readdirSync } from "node:fs";
import { join, resolve } from "node:path";
import { spawnSync } from "node:child_process";

const exampleRoot = process.cwd();
const sdkRoot = resolve(exampleRoot, "../../../sdk/typescript/worker");
const tscEntry = join(sdkRoot, "node_modules", "typescript", "bin", "tsc");

if (!existsSync(tscEntry)) {
  process.exit(0);
}

const packageDirs = [
  join(sdkRoot, "node_modules", "@relaymesh", "relaybus-core"),
  join(sdkRoot, "node_modules", "@relaymesh", "relaybus-amqp"),
  join(sdkRoot, "node_modules", "@relaymesh", "relaybus-kafka"),
  join(sdkRoot, "node_modules", "@relaymesh", "relaybus-nats"),
];

for (const pkgDir of packageDirs) {
  const srcDir = join(pkgDir, "src");
  const distIndex = join(pkgDir, "dist", "index.js");

  if (!existsSync(srcDir) || existsSync(distIndex)) {
    continue;
  }

  const srcFiles = readdirSync(srcDir)
    .filter((name) => name.endsWith(".ts"))
    .map((name) => join(srcDir, name));

  if (srcFiles.length === 0) {
    continue;
  }

  const result = spawnSync(
    process.execPath,
    [
      tscEntry,
      "--module",
      "commonjs",
      "--target",
      "es2020",
      "--moduleResolution",
      "node",
      "--esModuleInterop",
      "--skipLibCheck",
      "--declaration",
      "false",
      "--outDir",
      join(pkgDir, "dist"),
      ...srcFiles,
    ],
    {
      cwd: sdkRoot,
      stdio: "inherit",
    },
  );

  if (result.status !== 0) {
    process.exit(result.status ?? 1);
  }
}
