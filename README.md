# linear-cli

[![Go Version](https://img.shields.io/badge/Go-1.25-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![MIT License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![Release](https://img.shields.io/github/v/release/gluonfield/linear-cli?include_prereleases)](https://github.com/gluonfield/linear-cli/releases)
[![Zero Dependencies](https://img.shields.io/badge/deps-zero-success)](https://github.com/gluonfield/linear-cli)

Linear issue tracking, from your terminal. One static binary, 35+ commands, zero runtime dependencies.

```console
$ linear list --team ADI --status "In Progress"
ADI-12  Fix authentication redirect   In Progress  alice  High
ADI-34  Add rate limiting              In Progress  bob    Medium
ADI-37  Refactor GraphQL schema        In Progress  alice  High

$ linear create --title "Fix login redirect" --team ENG --priority 2
ENG-142 created
```

## Quick Start

```sh
go install github.com/gluonfield/linear-cli@latest

export LINEAR_API_KEY=lin_api_...

linear list --team YOUR_TEAM
```

Generate an API key at **Linear > Settings > API > Personal API keys**.

## Features

| Category        | Commands                                                            |
| --------------- | ------------------------------------------------------------------- |
| **Issues**      | `list` `get` `create` `update` `delete` `archive` `search` `comment` `batch-create` |
| **Projects**    | `projects` `project-create` `project-update`                        |
| **Cycles**      | `cycles` `cycle-create`                                             |
| **Initiatives** | `initiatives` `init-create`                                         |
| **Labels**      | `labels` `label-create` `label-delete`                              |
| **States**      | `states` `state-create`                                             |
| **Documents**   | `docs` `doc-create`                                                 |
| **Webhooks**    | `webhooks` `webhook-create` `webhook-delete`                        |
| **Notifications** | `notifications` `notif-archive` `notif-read`                      |
| **Users**       | `users` `me`                                                        |
| **Teams**       | `teams`                                                             |

## Installation

**Go install**

```sh
go install github.com/gluonfield/linear-cli@latest
```

**Pre-built binary**

Download from [Releases](https://github.com/gluonfield/linear-cli/releases).

## Usage

### Issues

```sh
linear list --team ADI --status "In Progress" --assignee "Alice" --limit 50
linear list -s "authentication bug" --team ENG
linear get ENG-142
linear update ENG-142 --status "Done" --assignee "Bob" --priority 3
linear update ENG-142 --due 2026-06-01T00:00:00Z
linear update ENG-142 --clear-due
linear delete ENG-142
linear archive ENG-142
linear unarchive ENG-142
```

### Search & Comments

```sh
linear search "API rate limit" --limit 10
linear comment ENG-142 "Fixed in commit abc123"
linear comments ENG-142
```

### Batch Create (stdin JSON lines)

```sh
echo '{"title":"Setup CI","priority":2}
{"title":"Write tests","priority":3}' | linear batch-create --team ENG
```

### Other Entities

```sh
linear projects --status "Planned"
linear cycles --team ENG
linear initiatives
linear labels --team ENG
linear states --team ENG
linear docs --team ENG
linear webhooks
linear notifications --limit 50
```

### Create & Delete

```sh
linear project-create --name "Q3 Launch" --team ENG --desc "Ship v2"
linear cycle-create --name "Sprint 42" --team ENG --start 2026-05-01T00:00:00Z --end 2026-05-14T23:59:59Z
linear init-create --name "Platform V3" --target 2026-09-01T00:00:00Z
linear label-create --name "security" --team ENG --color "#ff0000"
linear state-create --name "In Review" --team ENG --type "started" --color "#ffa500"
linear doc-create --title "Architecture RFC" --team ENG --desc "Proposal for..."
linear webhook-create --url https://example.com/webhook --team ENG
```

## Agentic Use

Designed for AI agents. Works with [psst](https://github.com/nicois/psst) for secret injection -- the API key never enters the agent's context.

```sh
psst --global LINEAR_API_KEY -- linear list --team ADI --status "In Progress"
```

**JSON output** for structured consumption:

```sh
psst --global LINEAR_API_KEY -- linear list --team ADI --json | jq '.[].title'
psst --global LINEAR_API_KEY -- linear users --json
psst --global LINEAR_API_KEY -- linear get ENG-142 --json
```

**Quiet mode** for scripts and pipelines:

```sh
psst --global LINEAR_API_KEY -- linear create --title "New bug" --team ENG -q
```

Returns only the issue identifier, no extra output.

## Development

```sh
git clone https://github.com/gluonfield/linear-cli.git
cd linear-cli
go build -o linear .
./linear --help
```

```
main.go              entry point
cmd/
  root.go            root command, auth check
  issues.go          list, search, filter flags
  create.go          issue create + helpers
  get.go             issue detail
  update.go          issue update
  delete.go          delete, archive, unarchive
  comment.go         add/list comments
  batch.go           batch-create from stdin
  teams.go           list teams
  labels.go          list/create/delete labels
  states.go          list/create workflow states
  projects.go        list/create/update projects
  cycles.go          list/create cycles
  initiatives.go     list/create initiatives
  documents.go       list/create documents
  users.go           list users
  me.go              current user
  webhooks.go        list/create/delete webhooks
  notifications.go   list/archive/read notifications
api/
  client.go          GraphQL HTTP client
```

## License

MIT
