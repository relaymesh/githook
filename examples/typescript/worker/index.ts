import {
  New,
  WithEndpoint,
  WithClientProvider,
  WithAPIKey,
  WithTenant,
  WithListener,
  NewRemoteSCMClientProvider,
  GitHubClient,
} from "@relaymesh/githook";

async function main() {
  const endpoint = process.env.GITHOOK_ENDPOINT ?? "https://relaymesh.vercel.app/api/connect";
  const ruleId = process.env.GITHOOK_RULE_ID ?? "85101e9f-3bcf-4ed0-b561-750c270ef6c3";
  const apiKey = process.env.GITHOOK_API_KEY ?? "";
  const tenantId = process.env.GITHOOK_TENANT_ID ?? "";
  console.log(
    `config endpoint=${endpoint} apiKeySet=${apiKey.length > 0} tenantId=${tenantId || "(empty)"}`,
  );

  const provider = NewRemoteSCMClientProvider();

  const options = [WithEndpoint(endpoint), WithClientProvider(provider)];
  options.push(
    WithListener({
      onMessageStart: (_ctx, evt) => {
        console.log(`listener start log_id=${evt.metadata?.log_id ?? ""} topic=${evt.topic}`);
      },
      onMessageFinish: (_ctx, evt, err) => {
        const status = err ? "failed" : "success";
        console.log(
          `listener finish log_id=${evt.metadata?.log_id ?? ""} status=${status} err=${err?.message ?? ""}`,
        );
      },
      onError: (_ctx, evt, err) => {
        console.log(
          `listener error log_id=${evt?.metadata?.log_id ?? ""} err=${err.message}`,
        );
      },
    }),
  );
  if (apiKey) {
    options.push(WithAPIKey(apiKey));
  }
  if (tenantId) {
    options.push(WithTenant(tenantId));
  }
  const wk = New(...options);

  wk.HandleRule(ruleId, async (_ctx, evt) => {
    if (!evt) return;
    console.log(`topic=${evt.topic} provider=${evt.provider} type=${evt.type}`);
    console.log(`metadata=${JSON.stringify(evt.metadata ?? {})}`);
    const payloadText = evt.payload?.toString("utf8") ?? "";
    if (payloadText) {
      try {
        const payloadJson = JSON.parse(payloadText);
        console.log(`payload=${JSON.stringify(payloadJson)}`);
      } catch {
        console.log(`payload=${payloadText}`);
      }
    }
    if (evt.normalized) {
      console.log(`normalized=${JSON.stringify(evt.normalized)}`);
    }

    const gh = GitHubClient(evt);
    if (!gh) {
      console.log(`github client not available for provider=${evt.provider} (installation may not be configured)`);
      return;
    }

    const { owner, repo } = repositoryFromEvent(evt);
    if (!owner || !repo) {
      console.log("repository info missing in payload; skipping github read");
      return;
    }

    try {
      const repository = await gh.requestJSON<Record<string, unknown>>(
        "GET",
        `/repos/${owner}/${repo}`,
      );
      console.log(
        `github read ok full_name=${repository.full_name} private=${repository.private} default_branch=${repository.default_branch}`,
      );
    } catch (err) {
      console.log(`github read failed owner=${owner} repo=${repo} err=${err}`);
    }
  });

  await wk.Run();
}

function repositoryFromEvent(evt: { normalized?: Record<string, unknown> }): {
  owner: string;
  repo: string;
} {
  const repoValue = evt.normalized?.["repository"];
  if (!repoValue || typeof repoValue !== "object") {
    return { owner: "", repo: "" };
  }

  const repoMap = repoValue as Record<string, unknown>;
  const fullName = repoMap["full_name"];
  if (typeof fullName === "string") {
    const parts = fullName.trim().split("/", 2);
    if (parts.length === 2 && parts[0] && parts[1]) {
      return { owner: parts[0], repo: parts[1] };
    }
  }

  const name = typeof repoMap["name"] === "string" ? repoMap["name"] : "";
  const ownerMap = repoMap["owner"];
  const ownerLogin =
    ownerMap && typeof ownerMap === "object"
      ? (ownerMap as Record<string, unknown>)["login"]
      : "";
  return {
    owner: typeof ownerLogin === "string" ? ownerLogin.trim() : "",
    repo: typeof name === "string" ? name.trim() : "",
  };
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
