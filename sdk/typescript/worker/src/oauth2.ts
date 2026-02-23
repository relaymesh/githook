import type { WorkerContext } from "./context.js";

export interface OAuth2Config {
  enabled?: boolean;
  issuer?: string;
  audience?: string;
  requiredScopes?: string[];
  requiredRoles?: string[];
  requiredGroups?: string[];

  mode?: string;
  clientId?: string;
  clientSecret?: string;
  scopes?: string[];

  redirectUrl?: string;
  authorizeUrl?: string;
  tokenUrl?: string;
  jwksUrl?: string;
}

interface CachedToken {
  token: string;
  expiresAt: number;
}

const tokenCache = new Map<string, CachedToken>();

export function resolveOAuth2Config(explicit?: OAuth2Config): OAuth2Config | undefined {
  if (explicit) {
    return explicit;
  }
  const tokenUrl = envValue("GITHOOK_OAUTH2_TOKEN_URL");
  if (!tokenUrl) {
    return undefined;
  }
  return {
    enabled: true,
    tokenUrl,
    clientId: envValue("GITHOOK_OAUTH2_CLIENT_ID"),
    clientSecret: envValue("GITHOOK_OAUTH2_CLIENT_SECRET"),
    scopes: splitCSV(envValue("GITHOOK_OAUTH2_SCOPES")),
    audience: envValue("GITHOOK_OAUTH2_AUDIENCE"),
  };
}

export async function oauth2TokenFromConfig(
  ctx: WorkerContext | undefined,
  cfg: OAuth2Config | undefined,
): Promise<string> {
  if (!cfg || cfg.enabled === false) {
    return "";
  }
  const tokenUrl = (cfg.tokenUrl ?? "").trim();
  const clientId = (cfg.clientId ?? "").trim();
  const clientSecret = (cfg.clientSecret ?? "").trim();
  if (!tokenUrl || !clientId || !clientSecret) {
    return "";
  }
  const cacheKey = buildCacheKey(cfg);
  const cached = tokenCache.get(cacheKey);
  const now = Date.now();
  if (cached && cached.token && cached.expiresAt > now + 30000) {
    return cached.token;
  }

  const body = new URLSearchParams();
  body.set("grant_type", "client_credentials");
  body.set("client_id", clientId);
  body.set("client_secret", clientSecret);
  if (cfg.scopes && cfg.scopes.length > 0) {
    body.set("scope", cfg.scopes.join(" "));
  }
  if (cfg.audience) {
    body.set("audience", cfg.audience);
  }

  const resp = await fetch(tokenUrl, {
    method: "POST",
    headers: { "Content-Type": "application/x-www-form-urlencoded" },
    body,
    signal: ctx?.signal,
  });
  if (!resp.ok) {
    const text = await resp.text().catch(() => "");
    throw new Error(`oauth2 token failed (${resp.status}): ${text}`);
  }
  const payload = (await resp.json()) as { access_token?: string; expires_in?: number };
  const token = (payload.access_token ?? "").trim();
  if (!token) {
    return "";
  }
  const expiresIn = payload.expires_in ?? 1800;
  tokenCache.set(cacheKey, {
    token,
    expiresAt: now + expiresIn * 1000,
  });
  return token;
}

function buildCacheKey(cfg: OAuth2Config): string {
  return [
    (cfg.tokenUrl ?? "").trim(),
    (cfg.clientId ?? "").trim(),
    (cfg.scopes ?? []).join(" "),
    (cfg.audience ?? "").trim(),
  ].join("|");
}

function envValue(key: string): string {
  return (process.env[key] ?? "").trim();
}

function splitCSV(value: string): string[] {
  return value
    .split(",")
    .map((entry) => entry.trim())
    .filter((entry) => entry.length > 0);
}
