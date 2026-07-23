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
life-ustc --server http://localhost:3000 account login

# Or set default server
life-ustc config set-server http://localhost:3000

# Authenticate
life-ustc account login
life-ustc account session
life-ustc account profile
life-ustc account locale zh-cn

# Personal workflows
life-ustc workspace overview
life-ustc workspace todo --pending
life-ustc workspace todo create --title "Write report" --priority high
life-ustc workspace todo complete <TODO_ID>
life-ustc workspace homework --pending
life-ustc workspace homework complete <HOMEWORK_ID>
life-ustc workspace schedule
life-ustc workspace exam
life-ustc workspace subscription list
life-ustc workspace subscription set <SECTION_ID_1> <SECTION_ID_2>
life-ustc workspace subscription import <CODE_1> <CODE_2>
life-ustc workspace calendar events
life-ustc workspace calendar feed
life-ustc workspace bus-preferences get
life-ustc workspace link-pin list
life-ustc workspace link-pin pin jw
life-ustc workspace upload create ./report.pdf
life-ustc workspace upload download <ID> -o report.pdf

# Browse (no auth required unless noted)
# In a terminal, bare course/section/teacher list commands open an interactive TUI.
life-ustc catalog course
life-ustc catalog course list --search "数学分析"
life-ustc catalog course --no-interactive --limit 20
life-ustc catalog course get <JW_ID>
life-ustc catalog section list --semester-id <ID>
life-ustc catalog teacher list --search "张"
life-ustc catalog semester current
life-ustc catalog bus timetable --from east --to west
life-ustc catalog link
life-ustc catalog metadata

# Official USTC sources
life-ustc workspace school semesters
life-ustc workspace school semesters --graduate
life-ustc workspace school curriculum --semester-id <ID>
life-ustc workspace school exam
life-ustc workspace school score
life-ustc workspace school homework
life-ustc workspace school sync --dry-run

# Community features
life-ustc community comment list --target-type section --target-id <ID>
life-ustc community comment create --target-type section --target-id <ID> --body "Great class!"
life-ustc community description get --target-type course --target-id <ID>
life-ustc community section-homework create <SECTION_ID> --title "Problem Set 1"

# Raw API access
life-ustc api catalog/semesters/current
life-ustc api workspace/todos -F title='Write report' -F priority=high
life-ustc api catalog/sections --jq '.data[].code'

# Admin
life-ustc admin user list
life-ustc admin comment list --status active
life-ustc admin suspension create --user-id <ID> --reason "spam"
```

## Command Model

- `catalog` contains public campus facts such as courses, sections, teachers, schedules, and buses.
- `workspace` contains the current user's todos, homework state, schedules, exams, subscriptions, files, and official-school imports.
- `community` contains shared comments, descriptions, and section homework entities.
- `account` contains profile, login, session, token, and locale operations.
- `admin` contains platform governance commands.
- `config`, `completion`, and `api` remain top-level CLI plumbing rather than product domains.
- Commands that benefit from guided input open their own TUI by default in an interactive terminal when no list/filter flags are provided, such as `course`, `section`, and `teacher`; use `--no-interactive` to force plain table output.

## Official USTC Sources

- `life-ustc workspace school semesters` reads undergraduate semesters from `catalog.ustc.edu.cn`, or graduate semesters from the official `yjs1.ustc.edu.cn` graduate apps with `--graduate`.
- `life-ustc workspace school curriculum`, `exam`, and `score` sign in directly from Go without a browser backend.
- `life-ustc workspace school homework` reads Blackboard or graduate homework data.
- `life-ustc workspace school sync` matches school-system lessons to Life@USTC sections and updates workspace subscriptions.

Authenticated `school` commands accept `--username`, `--password`, `--totp`, `--undergraduate`, `--graduate`, and sync commands also accept `--all-programs`. If omitted, the CLI falls back to the configured school program list, then program-specific credentials, then undergraduate. Persist defaults with `life-ustc config set-school-programs undergraduate,graduate`.

- Undergraduate username: `PASSPORT_UNDERGRADUATE_USERNAME`
- Graduate username: `PASSPORT_GRADUATE_USERNAME`
- Password: `PASSPORT_PASSWORD`
- TOTP: `PASSPORT_TOTP`

## JSON output

All commands support `--format json` or `--json` for machine-readable output:

```bash
life-ustc --json semester list
life-ustc --json catalog course get 12345
life-ustc catalog section list --jq '.data[].code'
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
# GET /api/catalog/metadata
life-ustc api catalog/metadata

# POST /api/workspace/todos with JSON body inferred from fields
life-ustc api workspace/todos -F title='Write report' -F priority=high

# POST exact request body from a file
life-ustc api -X POST workspace/todos --input ./todo.json

# Include response headers
life-ustc api -i catalog/metadata
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
