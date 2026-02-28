import os
import signal
import threading

from relaymesh_githook import (
    New,
    WithClientProvider,
    WithEndpoint,
    GitHubClient,
    NewRemoteSCMClientProvider,
)

stop = threading.Event()
attempts = 0


def shutdown(_signum, _frame):
    stop.set()


signal.signal(signal.SIGINT, shutdown)
signal.signal(signal.SIGTERM, shutdown)

endpoint = os.getenv("GITHOOK_ENDPOINT", "https://relaymesh.vercel.app/api/connect")
rule_id = os.getenv("GITHOOK_RULE_ID", "85101e9f-3bcf-4ed0-b561-750c270ef6c3")

wk = New(
    WithEndpoint(endpoint),
    WithClientProvider(NewRemoteSCMClientProvider()),
)


def repository_from_event(evt):
    normalized = getattr(evt, "normalized", None)
    if not normalized or not isinstance(normalized, dict):
        return "", ""
    repo_value = normalized.get("repository")
    if not isinstance(repo_value, dict):
        return "", ""
    full_name = repo_value.get("full_name", "")
    if isinstance(full_name, str) and "/" in full_name:
        parts = full_name.strip().split("/", 1)
        if len(parts) == 2 and parts[0] and parts[1]:
            return parts[0], parts[1]
    name = repo_value.get("name", "")
    owner_map = repo_value.get("owner")
    owner = owner_map.get("login", "") if isinstance(owner_map, dict) else ""
    return str(owner).strip(), str(name).strip()


def handle(ctx, evt):
    global attempts
    attempts += 1
    try:
        if attempts % 2 == 0:
            raise RuntimeError(f"intentional failure for status test (seq={attempts})")

        print(
            f"handler success seq={attempts} topic={evt.topic} provider={evt.provider} type={evt.type}"
        )
        print(f"topic={evt.topic} provider={evt.provider} type={evt.type}")

        gh = GitHubClient(evt)
        if not gh:
            print(
                f"github client not available for provider={evt.provider} (installation may not be configured)"
            )
            return

        owner, repo = repository_from_event(evt)
        if not owner or not repo:
            print("repository info missing in payload; skipping github read")
            return

        try:
            repository = gh.request_json("GET", f"/repos/{owner}/{repo}")
            if isinstance(repository, dict):
                full_name = repository.get("full_name")
                private = repository.get("private")
                default_branch = repository.get("default_branch")
            else:
                full_name = None
                private = None
                default_branch = None
            print(
                f"github read ok full_name={full_name} "
                f"private={private} "
                f"default_branch={default_branch}"
            )
        except Exception as err:
            print(f"github read failed owner={owner} repo={repo} err={err}")
    except Exception as err:
        print(f"handler failed seq={attempts} err={err}")
        raise


wk.HandleRule(rule_id, handle)

wk.Run(stop)
