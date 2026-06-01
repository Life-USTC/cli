# Life@USTC CLI

Command-line client for the [Life@USTC](https://life-ustc.tiankaima.dev) campus platform.

Built with Go around a [GitHub CLI](https://github.com/cli/cli)-style command model.

## Installation

### From release binaries

Download the latest release from [GitHub Releases](https://github.com/Life-USTC/CLI/releases).

### From source

```bash
go install github.com/Life-USTC/CLI/cmd/life-ustc@latest
```

### Build from source

```bash
git clone https://github.com/Life-USTC/CLI.git
cd CLI
make build
```

## Usage

```bash
# Set server (default: https://life-ustc.tiankaima.dev)
life-ustc --server http://localhost:3000 auth login

# Or set default server
life-ustc config set-server http://localhost:3000

# Authenticate
life-ustc auth login
life-ustc auth status
life-ustc me

# Personal workflows
life-ustc todo --pending
life-ustc todo create --title "Write report" --priority high
life-ustc todo done <TODO_ID>
life-ustc homework --pending
life-ustc homework create <SECTION_ID> --title "Problem Set 1"
life-ustc homework create
life-ustc homework done <HOMEWORK_ID>
life-ustc upload file ./report.pdf
life-ustc upload download <ID> -o report.pdf
life-ustc calendar get
life-ustc calendar set <SECTION_ID_1> <SECTION_ID_2>

# Browse (no auth required unless noted)
# In a terminal, bare course/section/teacher list commands open an interactive TUI.
life-ustc course
life-ustc course list --search "数学分析"
life-ustc course --no-interactive --limit 20
life-ustc course view <JW_ID>
life-ustc section list --semester-id <ID>
life-ustc section
life-ustc teacher list --search "张"
life-ustc teacher
life-ustc semester list
life-ustc semester current
life-ustc bus query --from east --to west
life-ustc metadata

# Official USTC sources
life-ustc school semesters
life-ustc school semesters --graduate
life-ustc school curriculum --semester-id <ID>
life-ustc school curriculum --graduate
life-ustc school exam
life-ustc school score
life-ustc school homework
life-ustc school sync --dry-run
life-ustc school sync --graduate --dry-run
life-ustc school sync

# Community features
life-ustc comment list --target-type section --target-id <ID>
life-ustc comment create --target-type section --target-id <ID> --body "Great class!"
life-ustc description get --target-type course --target-id <ID>
life-ustc description set --target-type course --target-id <ID> --content "Good for freshmen."

# Raw API access
life-ustc api semesters/current
life-ustc api todos -F title='Write report' -F priority=high
life-ustc api sections --jq '.data[].code'

# Admin
life-ustc admin user list
life-ustc admin comment list --status active
life-ustc admin suspension create --user-id <ID> --reason "spam"
```

## Command Model

- `me` is identity and account status.
- Personal resources live at the top level: `todo`, `homework`, `calendar`, `upload`.
- Browseable campus resources also live at the top level: `course`, `section`, `teacher`, `semester`, `schedule`, `bus`.
- `school` groups official USTC integrations: `semesters`, `curriculum`, `exam`, `score`, `homework`, and `sync`.
- Generic cross-resource workflows are available via `comment`, `description`, and `api`.
- Commands that benefit from guided input open their own TUI by default in an interactive terminal when no list/filter flags are provided, such as `course`, `section`, and `teacher`; use `--no-interactive` to force plain table output.

## Official USTC Sources

- `life-ustc school semesters` reads undergraduate semesters from `catalog.ustc.edu.cn`, or graduate semesters from the official `yjs1.ustc.edu.cn` graduate apps with `--graduate`.
- `life-ustc school curriculum`, `exam`, and `score` sign in directly from Go without a browser backend. Undergraduate data comes from `jw.ustc.edu.cn`; graduate data comes from the official `yjs1.ustc.edu.cn` graduate apps with `--graduate`.
- `life-ustc school homework` reads Blackboard calendar/homework data from `www.bb.ustc.edu.cn`, or graduate homework from the official `yjs1.ustc.edu.cn` graduate apps with `--graduate`.
- `life-ustc school sync` reads lesson codes from the active school system, matches them to Life@USTC sections, and updates your Life@USTC calendar subscriptions. The sync is one-way to Life@USTC only; it does not write back to USTC systems. Use `--dry-run` to preview matches without updating subscriptions.

Authenticated `school` commands accept `--username`, `--password`, `--totp`, `--undergraduate`, `--graduate`, and sync commands also accept `--all-programs`. If omitted, the CLI falls back to the configured school program list, then program-specific credentials, then undergraduate. Persist defaults with `life-ustc config set-school-programs undergraduate,graduate`.

- Undergraduate username: `PASSPORT_UNDERGRADUATE_USERNAME`
- Graduate username: `PASSPORT_GRADUATE_USERNAME`
- Password: `PASSPORT_PASSWORD`
- TOTP: `PASSPORT_TOTP`

## JSON output

All commands support `--format json` or `--json` for machine-readable output:

```bash
life-ustc --json semester list
life-ustc --json course view 12345
life-ustc section list --jq '.data[].code'
```

## Shell Integration

Install completion into your current shell without manually registering scripts:

```bash
life-ustc completion install
```

You can also target a specific shell:

```bash
life-ustc completion install --shell zsh
life-ustc completion install --shell bash
```

Manual script generation remains available for package managers or custom setups:

```bash
life-ustc completion -s zsh
life-ustc completion -s fish
```

## Raw API

Use `life-ustc api` for unsupported or newly added endpoints.

```bash
# GET /api/metadata
life-ustc api metadata

# POST /api/todos with JSON body inferred from fields
life-ustc api todos -F title='Write report' -F priority=high

# POST exact request body from a file
life-ustc api -X POST todos --input ./todo.json

# Include response headers
life-ustc api -i metadata
```

## Configuration

- Config directory: `~/.config/life-ustc/` (or `$XDG_CONFIG_HOME/life-ustc/`)
- Override server per-command: `--server URL`
- Environment variable: `LIFE_USTC_SERVER`
- School program default: `life-ustc config set-school-programs undergraduate,graduate`

## Global Options

| Option       | Description                    |
|-------------|--------------------------------|
| `--server`  | Server URL                     |
| `--format`  | Output format (table/json)     |
| `--jq`      | Filter JSON output with jq     |
| `--no-color`| Disable colored output         |
| `--version` | Show version                   |
| `--help`    | Show help                      |

## License

MIT
