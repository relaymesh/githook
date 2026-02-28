import { New, WithEndpoint } from "@relaymesh/githook";

async function main() {
  const endpoint = process.env.GITHOOK_ENDPOINT ?? "https://relaymesh.vercel.app/api/connect";
  const ruleId = process.env.GITHOOK_RULE_ID ?? "85101e9f-3bcf-4ed0-b561-750c270ef6c3";

  const wk = New(
    WithEndpoint(endpoint),
  );

  wk.HandleRule(ruleId, async (evt) => {
    console.log(`topic=${evt.topic} provider=${evt.provider} type=${evt.type}`);
  });

  await wk.Run();
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
