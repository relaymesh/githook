# GitHub Webhook Example

This example sends a GitHub pull_request webhook to the local githook server.

## Prerequisites
- githook running on `http://localhost:8080`
- `GITHUB_WEBHOOK_SECRET` set to the same value in your config
- Optional SCM auth: `GITHUB_APP_ID` and `GITHUB_PRIVATE_KEY_PATH`

## Run
```sh
export GITHUB_WEBHOOK_SECRET=devsecret
export GITHUB_APP_ID=123
export GITHUB_PRIVATE_KEY_PATH=/path/to/github.pem
./scripts/send_webhook.sh github pull_request example/github/pull_request.json
```

Tag event:
```sh
./scripts/send_webhook.sh github create example/github/tag_created.json
```

Push/commit event:
```sh
./scripts/send_webhook.sh github push example/github/push.json
```

Example config (rules) is in `example/github/app.yaml`.

## Alternate payload
Pass a JSON file to send a different payload:
```sh
./scripts/send_webhook.sh github pull_request /path/to/your.json
```

## Notes
- The example uses `X-Hub-Signature` (HMAC SHA-1) which is required by the current webhook parser.
- The worker handles multiple GitHub event types: `push`, `check_suite`, `pull_request`, etc.
- For commit events, the worker automatically extracts `head_commit` from both `push` events (top level) and `check_suite` events (nested in `check_suite.head_commit`).
- Set your rules to match the payload, for example:
```yaml
rules:
  # Pull request opened
  - when: action == "opened" && pull_request.draft == false
    emit: pr.opened.ready

  # Push events (single commit)
  - when: head_commit.id != "" && commits[0].id != "" && commits[1] == null
    emit: github.commit.created

  # Check suite events (also contain commit information)
  - when: action == "requested" && check_suite.head_commit.id != ""
    emit: github.commit.created
```

## Worker Example

This worker subscribes to `pr.opened.ready`, `pr.merged`, and `github.commit.created` topics and runs custom logic.

### Running the Worker

```sh
go run ./example/github/worker --config config.yaml --driver amqp
```

The example uses `worker.NewSCMClientProvider` to return the official GitHub SDK client
(`go-github`) without any manual client construction.

To target a single subscriber driver (when `watermill.drivers` is set), pass `-driver`:
```sh
go run ./example/github/worker -driver amqp
```

The worker reads the `watermill` section from your app config, so you can reuse the same YAML
you run the server with.

### Testing GitHub Client with Commits

The worker includes a comprehensive test for the GitHub API client using the `github.commit.created` topic. When you push commits to a repository with a GitHub App installed, GitHub sends both `push` and `check_suite` events. The worker handles both event structures automatically.

**Send a test push/commit event:**
```sh
./scripts/send_webhook.sh github push example/github/push.json
```

**What the test does:**

1. ✅ Extracts repository information (`owner/repo`) from the push event
2. ✅ Extracts commit ID and message from the event payload
3. ✅ **Uses GitHub API client** to fetch repository details:
   - Repository name
   - Star count
   - Fork count
4. ✅ **Uses GitHub API client** to fetch commit details:
   - Commit SHA
   - Author information
   - Files changed
   - Code statistics (additions, deletions, total)

**Expected output:**
```
githook/worker-example github.commit.created: repo=yindia/test-repo commit=a1b2c3d4... message="Add new feature for testing githook"
githook/worker-example Testing GitHub client: fetching repository yindia/test-repo
githook/worker-example Repository info: name=test-repo stars=10 forks=2
githook/worker-example Testing GitHub client: fetching commit a1b2c3d
githook/worker-example Commit details: sha=a1b2c3d author=yindia files_changed=2
githook/worker-example Commit stats: additions=50 deletions=10 total=60
githook/worker-example ✅ GitHub client test passed for commit a1b2c3d
```

**Note:** The GitHub API calls will only work if:
- You have a GitHub App installed on the repository
- The app has read permissions for repository and contents
- Installation credentials are properly configured in your `config.yaml`

### Topics Subscribed

The worker handles three types of events:

- **`pr.opened.ready`**: Pull request opened (not draft)
- **`pr.merged`**: Pull request merged
- **`github.commit.created`**: Commit pushed (tests GitHub API client)

### GitHub SDK Usage

The worker demonstrates how to use the official GitHub SDK client:

```go
// The client is injected by the worker SDK
if evt.Client != nil {
    ghClient := evt.Client.(*github.Client)

    // Use the GitHub SDK
    repo, _, err := ghClient.Repositories.Get(ctx, owner, repoName)
    commit, _, err := ghClient.Repositories.GetCommit(ctx, owner, repoName, sha, nil)
}
```

The client is automatically authenticated using the GitHub App installation token.
