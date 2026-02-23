import { oauth2TokenFromConfig } from "./oauth2.js";
import type { WorkerContext } from "./context.js";
import type { OAuth2Config } from "./oauth2.js";

export interface RuleRecord {
  id: string;
  when: string;
  emit: string[];
  driverId: string;
}

export interface DriverRecord {
  id: string;
  name: string;
  configJson: string;
  enabled: boolean;
}

export interface APIClientOptions {
  baseUrl: string;
  apiKey?: string;
  oauth2Config?: OAuth2Config;
  tenantId?: string;
}

export class RulesClient {
  constructor(private readonly opts: APIClientOptions) {}

  async listRules(ctx?: WorkerContext): Promise<RuleRecord[]> {
    const payload = await postJSON<Record<string, unknown>>(
      this.opts,
      "/cloud.v1.RulesService/ListRules",
      {},
      ctx,
    );
    const rawRules = readArray(payload, "rules");
    return rawRules.map((record) => ({
      id: readString(record, "id"),
      when: readString(record, "when"),
      emit: readStringArray(record, "emit"),
      driverId: readString(record, "driver_id", "driverId"),
    }));
  }

  async getRule(id: string, ctx?: WorkerContext): Promise<RuleRecord> {
    const trimmed = (id ?? "").trim();
    if (!trimmed) {
      throw new Error("rule id is required");
    }
    const payload = await postJSON<Record<string, unknown>>(
      this.opts,
      "/cloud.v1.RulesService/GetRule",
      { id: trimmed },
      ctx,
    );
    const record = readObject(payload, "rule");
    if (!record) {
      throw new Error(`rule not found: ${trimmed}`);
    }
    return {
      id: readString(record, "id"),
      when: readString(record, "when"),
      emit: readStringArray(record, "emit"),
      driverId: readString(record, "driver_id", "driverId"),
    };
  }
}

export class DriversClient {
  constructor(private readonly opts: APIClientOptions) {}

  async listDrivers(ctx?: WorkerContext): Promise<DriverRecord[]> {
    const payload = await postJSON<Record<string, unknown>>(
      this.opts,
      "/cloud.v1.DriversService/ListDrivers",
      {},
      ctx,
    );
    const rawDrivers = readArray(payload, "drivers");
    return rawDrivers.map((record) => normalizeDriver(record));
  }

  async getDriverById(id: string, ctx?: WorkerContext): Promise<DriverRecord | undefined> {
    const trimmed = (id ?? "").trim();
    if (!trimmed) {
      throw new Error("driver id is required");
    }
    const drivers = await this.listDrivers(ctx);
    return drivers.find((record) => record.id === trimmed);
  }
}

export class EventLogsClient {
  constructor(private readonly opts: APIClientOptions) {}

  async updateStatus(logId: string, status: string, errorMessage?: string, ctx?: WorkerContext): Promise<void> {
    const trimmed = (logId ?? "").trim();
    if (!trimmed) {
      throw new Error("log id is required");
    }
    const statusVal = (status ?? "").trim();
    if (!statusVal) {
      throw new Error("status is required");
    }
    await postJSON(
      this.opts,
      "/cloud.v1.EventLogsService/UpdateEventLogStatus",
      {
        log_id: trimmed,
        status: statusVal,
        error_message: (errorMessage ?? "").trim(),
      },
      ctx,
    );
  }
}

export interface InstallationRecord {
  provider: string;
  accountId: string;
  accountName: string;
  installationId: string;
  providerInstanceKey: string;
  enterpriseId?: string;
  enterpriseSlug?: string;
  enterpriseName?: string;
  accessToken?: string;
  refreshToken?: string;
  expiresAt?: Date;
}

export class InstallationsClient {
  constructor(private readonly opts: APIClientOptions) {}

  async getByInstallationId(
    provider: string,
    installationId: string,
    ctx?: WorkerContext,
  ): Promise<InstallationRecord | undefined> {
    const trimmedProvider = (provider ?? "").trim();
    const trimmedID = (installationId ?? "").trim();
    if (!trimmedProvider) {
      throw new Error("provider is required");
    }
    if (!trimmedID) {
      throw new Error("installation_id is required");
    }
    const payload = await postJSON<Record<string, unknown>>(
      this.opts,
      "/cloud.v1.InstallationsService/GetInstallationByID",
      { provider: trimmedProvider, installation_id: trimmedID },
      ctx,
    );
    const record = readObject(payload, "installation");
    if (!record) {
      return undefined;
    }
    return normalizeInstallation(record);
  }
}

export interface SCMClientRecord {
  provider: string;
  apiBaseUrl: string;
  accessToken: string;
  providerInstanceKey: string;
  expiresAt?: Date;
}

export class SCMClientsClient {
  constructor(private readonly opts: APIClientOptions) {}

  async getSCMClient(
    provider: string,
    installationId: string,
    providerInstanceKey?: string,
    ctx?: WorkerContext,
  ): Promise<SCMClientRecord> {
    const trimmedProvider = (provider ?? "").trim();
    const trimmedID = (installationId ?? "").trim();
    if (!trimmedProvider) {
      throw new Error("provider is required");
    }
    if (!trimmedID) {
      throw new Error("installation_id is required");
    }
    const payload = await postJSON<Record<string, unknown>>(
      this.opts,
      "/cloud.v1.SCMService/GetSCMClient",
      {
        provider: trimmedProvider,
        installation_id: trimmedID,
        provider_instance_key: (providerInstanceKey ?? "").trim(),
      },
      ctx,
    );
    const record = readObject(payload, "client");
    if (!record) {
      throw new Error("scm client missing in response");
    }
    return normalizeSCMClient(record);
  }
}

function normalizeDriver(record: Record<string, unknown>): DriverRecord {
  return {
    id: readString(record, "id"),
    name: readString(record, "name"),
    configJson: readString(record, "config_json", "configJson"),
    enabled: readBool(record, "enabled"),
  };
}

async function postJSON<T>(
  opts: APIClientOptions,
  path: string,
  body: Record<string, unknown>,
  ctx?: WorkerContext,
): Promise<T> {
  const baseUrl = normalizeBaseUrl(opts.baseUrl);
  if (!baseUrl) {
    throw new Error("base url is required");
  }
  const url = `${baseUrl}${path}`;
  const headers: Record<string, string> = {
    "Content-Type": "application/json",
  };
  await applyAuthHeaders(headers, opts, ctx);
  const tenantId = (ctx?.tenantId ?? opts.tenantId ?? "").trim();
  if (tenantId) {
    headers["X-Tenant-ID"] = tenantId;
  }
  const resp = await fetch(url, {
    method: "POST",
    headers,
    body: JSON.stringify(body ?? {}),
    signal: ctx?.signal,
  });
  if (!resp.ok) {
    const text = await resp.text().catch(() => "");
    throw new Error(`request failed (${resp.status}): ${text}`);
  }
  const text = await resp.text();
  if (!text) {
    return {} as T;
  }
  return JSON.parse(text) as T;
}

async function applyAuthHeaders(
  headers: Record<string, string>,
  opts: APIClientOptions,
  ctx?: WorkerContext,
): Promise<void> {
  const apiKey = (opts.apiKey ?? "").trim();
  if (apiKey) {
    headers["x-api-key"] = apiKey;
    return;
  }
  try {
    const token = await oauth2TokenFromConfig(ctx, opts.oauth2Config);
    if (token) {
      headers["Authorization"] = `Bearer ${token}`;
    }
  } catch {
    return;
  }
}

function normalizeBaseUrl(baseUrl: string): string {
  return (baseUrl ?? "").trim().replace(/\/+$/, "");
}

function readArray(payload: Record<string, unknown> | undefined, key: string): Array<Record<string, unknown>> {
  if (!payload) {
    return [];
  }
  const value = payload[key];
  if (!Array.isArray(value)) {
    return [];
  }
  return value.filter((entry) => entry && typeof entry === "object") as Array<Record<string, unknown>>;
}

function readObject(payload: Record<string, unknown> | undefined, key: string): Record<string, unknown> | undefined {
  if (!payload) {
    return undefined;
  }
  const value = payload[key];
  if (value && typeof value === "object") {
    return value as Record<string, unknown>;
  }
  return undefined;
}

function readString(record: Record<string, unknown>, ...keys: string[]): string {
  for (const key of keys) {
    const value = record[key];
    if (typeof value === "string") {
      return value;
    }
  }
  return "";
}

function readBool(record: Record<string, unknown>, key: string): boolean {
  const value = record[key];
  if (typeof value === "boolean") {
    return value;
  }
  return false;
}

function readStringArray(record: Record<string, unknown>, key: string): string[] {
  const value = record[key];
  if (!Array.isArray(value)) {
    return [];
  }
  return value.filter((entry) => typeof entry === "string") as string[];
}

function normalizeInstallation(record: Record<string, unknown>): InstallationRecord {
  return {
    provider: readString(record, "provider"),
    accountId: readString(record, "account_id", "accountId"),
    accountName: readString(record, "account_name", "accountName"),
    installationId: readString(record, "installation_id", "installationId"),
    providerInstanceKey: readString(record, "provider_instance_key", "providerInstanceKey"),
    enterpriseId: readString(record, "enterprise_id", "enterpriseId"),
    enterpriseSlug: readString(record, "enterprise_slug", "enterpriseSlug"),
    enterpriseName: readString(record, "enterprise_name", "enterpriseName"),
    accessToken: readString(record, "access_token", "accessToken"),
    refreshToken: readString(record, "refresh_token", "refreshToken"),
    expiresAt: parseDate(record, "expires_at", "expiresAt"),
  };
}

function normalizeSCMClient(record: Record<string, unknown>): SCMClientRecord {
  return {
    provider: readString(record, "provider"),
    apiBaseUrl: readString(record, "api_base_url", "apiBaseUrl"),
    accessToken: readString(record, "access_token", "accessToken"),
    providerInstanceKey: readString(record, "provider_instance_key", "providerInstanceKey"),
    expiresAt: parseDate(record, "expires_at", "expiresAt"),
  };
}

function parseDate(record: Record<string, unknown>, ...keys: string[]): Date | undefined {
  for (const key of keys) {
    const value = record[key];
    if (typeof value === "string" && value) {
      const date = new Date(value);
      if (!Number.isNaN(date.getTime())) {
        return date;
      }
    }
    if (value && typeof value === "object" && "seconds" in value) {
      const seconds = Number((value as { seconds?: string | number }).seconds);
      if (!Number.isNaN(seconds)) {
        return new Date(seconds * 1000);
      }
    }
  }
  return undefined;
}
