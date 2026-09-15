# Life@USTC CLI

终端里使用 [Life@USTC](https://life-ustc.tiankaima.dev) 的命令行客户端
（`life-ustc`）。数据与权限来自
[server](https://github.com/Life-USTC/server)；域与能力名与 Web / Bot / MCP 同一棵树
（[interface hierarchy](https://github.com/Life-USTC/server/blob/main/docs/interface-hierarchy.md)）。

## 面向谁

- 习惯 shell 管理课表、待办、作业与订阅的同学
- 需要脚本化调用 REST、或从官方教务站同步数据的用户
- 做内容治理的管理员

## 命令域

| 域 | 能做什么 |
|----|----------|
| `catalog` | 公开事实：学期、课程、教学班、教师、课表、校车、校园链接、元数据、第二课堂活动与主办方、天气、教室地图、新闻公告 |
| `workspace` | 个人概览、完整个人日历 / iCal、课表、考试、待办 CRUD、作业完成态、教学班订阅、第二课堂活动与主办方订阅、提醒通知、校车偏好、链接置顶、上传 |
| `workspace school` | 直连校方站点：本科/研究生学期、课表、考试、成绩、作业，并可 `sync` 回 Life@USTC 订阅 |
| `community` | 评论（含反应）、描述、教学班作业、公开用户资料 |
| `account` | 登录 / 登出、session、token、资料、语言、当前客户端活动 |
| `admin` | 用户、封禁、评论 / 描述 / 作业治理 |
| `api` | 对任意 REST 路径的逃生舱（适合脚本） |
| `config` / `completion` | 默认 server、教务程序偏好、shell 补全 |

交互终端下，裸的 `course` / `section` / `teacher` 列表会打开 TUI；加过滤或
`--no-interactive` 则输出表格。机器可读输出：`--json` / `--format json`，可用 `--jq`。

登录支持浏览器 OAuth（PKCE）与设备码；默认 server 为生产站点，也可用
`--server` / `LIFE_USTC_SERVER` 指向其它实例。

常用公开与账户命令示例：

```bash
life-ustc catalog young-event --active true --limit 20
life-ustc catalog young-organizer list --search 学生会
life-ustc catalog young-event date week 2026-09-15
life-ustc workspace calendar events --date-from 2026-09-01 --date-to 2026-09-30
life-ustc workspace young-event-subscription set <young-id> --subscribed true --remind-start true
life-ustc catalog weather --location-key ustc-main
life-ustc catalog room map <room-code>
life-ustc catalog publication --type notice --limit 20
life-ustc workspace subscription kind <section-jw-id> teaching_assistant
life-ustc account client activity --limit 20
life-ustc community user get <username-or-id>
```

当前客户端活动需要 `account.client-activity:read` 权限。旧版本登录的用户需运行
`life-ustc account login` 重新授权，刷新旧 token 不会增加权限。

`workspace calendar events` 使用完整个人日历 REST 接口，默认读取当前上海日期起的
七天窗口；使用成对的 `--date-from` / `--date-to` 查询其它包含端点的日期范围。
Young 活动的 `catalog young-event date day|week|month` 会遍历该范围的所有分页。
要订阅 iCal 日历，请使用 `workspace calendar feed`，并将返回的 URL 导入日历应用。

## OpenAPI 契约

CLI 从仓库内的 `api/openapi.json` 生成。`api/openapi.provenance.json` 记录
对应的 server 提交和 SHA-256；`make build` 会先验证来源并重新生成客户端，
再开始编译。

更新契约时，先检出确定的 `Life-USTC/server` 提交，再运行：

```sh
make sync-openapi OPENAPI_SERVER_DIR=/path/to/server SERVER_COMMIT=<40-character-sha>
make generate
```

CI 会核对固定的 server 提交，不再猜测同名分支。定时同步工作流也会在
server 契约发生变化时创建更新 PR。

## 安装

发布包：[GitHub Releases](https://github.com/Life-USTC/CLI/releases)。或：

```bash
go install github.com/Life-USTC/CLI/cmd/life-ustc@latest
```

子命令细节以 `life-ustc <cmd> --help` 为准。License: MIT。
