---
name: linear-cli
description: Read, create, update, comment on Linear issues from the shell. Use whenever the user asks to triage, file, assign, status-check, or delegate work in Linear.
---

# Linear CLI — Agent Usage

Binary: `linear` (Go, single static binary). Auth: either `LINEAR_API_KEY` env var, or OAuth (`linear oauth setup && linear oauth login` once; token cached at `~/.config/linear-cli/auth.json`). API key wins if both are set.

## Always pass `--json`

Every command supports `--json`. Use it. Parse with `jq`. Never scrape table output.

## Discover identifiers first

Team keys (`ENG`, `ADI`) and project names are required by most write commands. If you don't know them, look them up:

```sh
linear teams --json | jq '.[].key'
linear projects --json | jq '.[].name'
linear me --json                              # your user id + email
```

Cache these for the session — don't re-fetch on every call.

## Read

```sh
linear list --team ENG --json -n 50
linear list --project "Q3 Launch" --status "In Progress" --json
linear list --assignee me --json
linear list -s "rate limit" --json            # full-text search
linear get ENG-142 --json                     # includes description AND comments inline
```

`get` returns the full issue + up to 100 comments in one call. Prefer it over `list` + per-issue `comments`.

## Write

```sh
linear create --team ENG --title "..." --desc "..." --project "Q3 Launch" \
              --assignee "Alice" --priority 2 --json
linear update ENG-142 --status "Done" --assignee me --json
linear update ENG-142 --project "Backlog"     # move between projects
linear update ENG-142 --clear-project         # detach from project
linear comment ENG-142 "deployed in abc123"
```

Priority: 1=urgent, 2=high, 3=medium, 4=low.
Status names are fuzzy-matched within the team's workflow.
`--assignee me` resolves to the API key's owner.

## Batch create

JSON lines on stdin, one issue per line:

```sh
cat <<'EOF' | linear batch-create --team ENG --project "Q3 Launch" --json
{"title": "Setup CI", "priority": 2}
{"title": "Write tests", "priority": 3, "assignee": "Alice"}
{"title": "Other-project task", "project": "Platform V3"}
EOF
```

Per-line `project`/`assignee` override the `--project` default.

## Idioms

```sh
# Pick highest-priority unassigned ticket in a project
linear list --project "Q3 Launch" --json \
  | jq '[.[] | select(.assignee == null)] | sort_by(.priority) | .[0].identifier'

# Comment on every issue matching a search
for id in $(linear list -s "flaky" --json | jq -r '.[].identifier'); do
  linear comment "$id" "Investigating — see #incident-123"
done

# Bulk-close a sprint's worth of tickets
linear list --project "Sprint 42" --status "In Review" --json \
  | jq -r '.[].identifier' \
  | xargs -I{} linear update {} --status "Done"
```

## Failure modes to handle

- `team "FOO" not found` → call `linear teams` and surface valid keys to the user
- `project "FOO" matched multiple: …` → ask the user which, or pass the UUID
- `status "X" not found. Available: …` → status names are team-specific; list `linear states --team ENG`
- Missing `LINEAR_API_KEY` → exits with auth error; do not retry, ask the user to set it

## Don't

- Don't write the API key to files, logs, or commit messages.
- Don't loop `list` to paginate manually — use `-n` (max ~250).
- Don't parse table output. Always `--json`.
- Don't impersonate humans in comments — write as yourself ("agent-X: ..." or sign comments) since API key auth attributes everything to the key's owner.
