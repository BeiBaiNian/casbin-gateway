<h1 align="center" style="border-bottom: none;">📦⚡️ Casbin Gateway</h1>
<h3 align="center">一个开源网关，管理你机器上的 AI 编程 Agent 与 Web 流量，由 Go 和 React 开发。</h3>
<p align="center">
  <a href="https://github.com/apache/casbin-gateway/actions/workflows/golangci-lint.yml">
    <img alt="Lint" src="https://github.com/apache/casbin-gateway/actions/workflows/golangci-lint.yml/badge.svg">
  </a>
  <a href="https://github.com/apache/casbin-gateway/actions/workflows/build.yml">
    <img alt="Build" src="https://github.com/apache/casbin-gateway/actions/workflows/build.yml/badge.svg">
  </a>
  <a href="https://pkg.go.dev/github.com/apache/casbin-gateway">
    <img alt="Go Reference" src="https://pkg.go.dev/badge/github.com/apache/casbin-gateway.svg">
  </a>
  <a href="https://github.com/apache/casbin-gateway/releases/latest">
    <img alt="GitHub Release" src="https://img.shields.io/github/v/release/apache/casbin-gateway.svg">
  </a>
</p>

<p align="center">
  <a href="https://github.com/apache/casbin-gateway/blob/master/LICENSE">
    <img alt="license" src="https://img.shields.io/github/license/apache/casbin-gateway">
  </a>
  <a href="https://github.com/apache/casbin-gateway/issues">
    <img alt="GitHub issues" src="https://img.shields.io/github/issues/apache/casbin-gateway">
  </a>
  <a href="https://github.com/apache/casbin-gateway/stargazers">
    <img alt="GitHub stars" src="https://img.shields.io/github/stars/apache/casbin-gateway">
  </a>
  <a href="https://github.com/apache/casbin-gateway/network">
    <img alt="GitHub forks" src="https://img.shields.io/github/forks/apache/casbin-gateway">
  </a>
  <a href="https://discord.gg/S5UjpzGZjN">
    <img alt="Discord" src="https://img.shields.io/discord/1022748306096537660?logo=discord&label=discord&color=5865F2">
  </a>
</p>

<p align="center">
  <a href="./README.md">English</a> | <b>中文</b>
</p>

## 运行

一条命令。不需要数据库，不需要 Go，不需要 Node，不需要配置。

Linux 和 macOS：

```bash
curl -fsSL https://raw.githubusercontent.com/apache/casbin-gateway/master/scripts/install.sh | bash
```

Windows，在 PowerShell 中：

```powershell
irm https://raw.githubusercontent.com/apache/casbin-gateway/master/scripts/install.ps1 | iex
```

两者都会下载适配本机的构建产物，解压到 `~/.local/share/casbin-gateway`（Windows 上是 `%LOCALAPPDATA%\casbin-gateway`），把 `casbin-gateway` 命令加入 PATH，然后启动它。接着打开：

**http://localhost:17000** —— 用 `admin` 和密码 `123` 登录，然后在 **My Account** 里改掉它。

安装到此为止。Gateway 把数据存在自己目录下的一个 SQLite 文件里，登录也走它自己的用户表。

### 接下来做什么

| 页面 | 你能得到什么 | 需要什么 |
| --- | --- | --- |
| **Agents** | 本机安装的每一个 AI 编程 Agent —— Claude Code、Codex CLI、Cursor 等等。点其中一个的 **Patch**，它的活动就会实时流进页面。 | 无 |
| **Skills & MCP** | 所有 Agent 的所有技能和 MCP 服务器汇总在一张表里。可以给一个或多个 Agent 添加 MCP 服务器，也可以打开、删除，或复制到另一个 Agent。 | 无 |
| **Providers** | 挡在模型厂商前面的统一入口。API Key 由 Gateway 持有，Agent 拿不到；也可以转发 Agent 自己的登录，什么都不持有。 | 一个厂商的 API Key，或者什么都不用 |
| **LLM Records** | Agent 转发的每一次请求：完整的 system prompt、每一条消息和工具调用、模型可用的每个工具的 schema，以及 token 数和费用。 | 一个 Provider，以及 `llmRecordMode` —— 见[记录提示词](#记录提示词) |
| **Advanced → Sites** | 反向代理 WAF：按站点的路由、规则、证书和统计。 | 打开代理 —— 见[开启 WAF 反向代理](#开启-waf-反向代理) |

Agent 是通过读取 **Gateway 所在机器**的用户账户、home 目录和安装路径发现的，所以要在你想观察的那台机器上运行它。

### 让 Agent 的流量走 Gateway

这是 **LLM Records** 的数据来源，也是让 Gateway 而不是 Agent 持有厂商 Key 的方式。

1. **Providers** → **Add**：选类型（OpenAI 兼容或 Anthropic 兼容），填厂商的 base URL 和 API Key，并列出它提供的模型。
2. **Agents** → 打开一个 Agent → 选中该 Provider。
3. 复制页面显示的环境变量片段，在设置了这些变量的 shell 里启动 Agent：

```bash
export ANTHROPIC_BASE_URL="http://localhost:17000/v1/agents/claude-code"
export ANTHROPIC_AUTH_TOKEN="casbin-gateway"
```

这个 token 只是占位符 —— Agent 没有它就拒绝启动，而 Gateway 会用 Provider 自己的 Key 去认证上游。

### 没有 API Key：沿用 Agent 已有的登录

用 ChatGPT 或 Claude 订阅登录的 Agent 根本没有 API Key 可填。把 Provider 的**认证方式**设成**调用方自己的登录**，它就不需要 Key：base URL 指向厂商，每个请求都带着 Agent 自己发来的凭据转发上游，Agent 继续用它已有的登录。**Models** 留空，这个 Provider 就接受任何模型名。

这种 Provider 的环境变量片段只设 base URL，不设别的 —— 在这里再设一个 token 会覆盖 Agent 已有的登录。记录和路由和有 Key 的 Provider 完全一样，只是 Gateway 从头到尾没见过任何 Key。

Codex 是例外：它的 ChatGPT 登录走的是另一套 API，不是 Gateway 转发的 chat completions，所以 Codex CLI 仍然需要一个带 API Key 的 Provider。

### 停止、升级、卸载

- **停止**：`Ctrl-C`。**再次启动**：在任意目录执行 `casbin-gateway` —— 这个命令是一个包装脚本，总是在安装目录（数据所在的地方）里启动 Gateway。
- **升级**：再跑一遍安装命令。数据库和设置不受影响。
- **卸载**：删除 `~/.local/share/casbin-gateway` 和 `~/.local/bin/casbin-gateway`（Windows 上是 `%LOCALAPPDATA%\casbin-gateway` 及其 PATH 条目）。

设置 `INSTALL_DIR` 可以装到别的位置，`NO_START=1` 则只安装不启动。

**这些是 nightly 构建**，每次推送都从 `master` 重新构建，并作为 [`nightly`](https://github.com/apache/casbin-gateway/releases/tag/nightly) 预发布版本发布。它们的用途是让人不装 Go 和 Node 工具链就能试用 Gateway；其他场景都应该从源码发布版构建。

### 在 Docker 或 Podman 里运行

**容器看不到你机器上的 Agent。** Agent 是靠读取 Gateway 所在机器的用户主目录和安装路径发现的，而在容器里那是容器自己的文件系统。所以 **Agents**、**Skills & MCP** 和 Agent 监控在容器里一直是空的，页面会直接说明这一点，而不是让人以为什么都没装。不依赖宿主机的部分照常可用：**Providers**、**LLM Records** 和 WAF 反向代理。

因此，想监控哪台机器上的 Agent，就在那台机器上跑上面的一键安装；只把 Gateway 当作模型入口或别的机器的反向代理时，才用容器部署。

我们没有发布镜像，compose 文件会用本仓库的源码构建一个，所以先把仓库 clone 下来，在仓库根目录执行：

```bash
docker compose up -d
```

Podman 读同一个文件：

```bash
podman compose up -d
```

两种方式下管理界面都在 http://localhost:17000，SQLite 数据库存在一个命名卷里，`down` 之后依然保留；`conf/app.conf` 从仓库挂载进去，首次启动前可以先改它播下的那份初始配置，不用重新构建镜像。

想在容器里用 WAF 反向代理，在 **设置** 页面里打开反向代理，并在 `docker-compose.yml` 里 `17000:17000` 旁边补上它的端口：

```yaml
    ports:
      - "17000:17000"
      - "8080:80"
      - "8443:443"
```

## 配置

所有配置都是可选的。设置在 Web UI 的 **设置** 页面里修改，保存在数据库中，既不用手工改文件，也不用重启。可执行文件旁边的 `conf/app.conf` 只在第一次启动时播下这些值，每一项在文件里都有说明；一步安装装出来的单个可执行文件旁边没有这个文件，播的是编进二进制里的那一份。第一次启动之后再改这个文件不会有任何效果，只有在数据库打开之前就要读的那几项例外：`httpport`、`driverName`、`dataSourceName`、`dbName` 和 `redisEndpoint`。真正常被改动的是这些：

| 配置项 | 默认值 | 作用 |
| --- | --- | --- |
| `httpport` | `17000` | Web UI 和 REST API 的端口 |
| `driverName` / `dataSourceName` | `sqlite` / `./data/casbin-gateway.db` | 数据存放位置 |
| `gatewayEnabled` | `false` | 打开反向代理 WAF |
| `gatewayHttpPort` / `gatewayHttpsPort` | `80` / `443` | 代理监听的端口 |
| `llmRecordMode` | `off` | 每次转发的 LLM 请求保留多少内容 |
| `apiKeyEncryptionKey` | 空 | 加密存储 Provider 的 API Key（AES-256-GCM） |
| `casdoorEndpoint` | 空 | 把登录切换到 [Casdoor](https://casdoor.org) SSO |

Gateway 启动时会打印它实际在做什么，所以可以直接看结果而不用去查配置文件：

```
+----------------------------------------------------------------------------+
| Casbin Gateway                                                              |
+----------------+-----------------------------------------------------------+
| Management UI  | http://localhost:17000                                     |
| Settings       | Settings page, seeded from conf/app.conf                   |
| Web UI files   | web/build                                                  |
| Reverse proxy  | enabled                                                    |
| Gateway HTTP   | :8080                                                      |
| Gateway HTTPS  | :8443                                                      |
| Database       | sqlite, file "./data/casbin-gateway.db" (connected)        |
| Sign-in        | built-in user table, Casdoor is not configured             |
| App dir        | ./data/apps                                                |
+----------------+-----------------------------------------------------------+
```

如果端口被占用，Gateway 会指出是哪个进程占着它并停止启动，而不是半配置地跑起来。

### 记录提示词

在你主动要求之前，转发请求的内容一概不存储，因为提示词里可能粘进任何东西。LLM Records 页面上的 **只记录元数据** 和 **记录元数据和正文** 两个按钮会从下一次请求起打开记录；设置页面里有同样的选项和相关的上限，它们的初始值来自：

```ini
; "off" 什么都不留，"metadata" 记录谁调用了哪个模型、结果如何，
; "full" 还会存下请求体 —— 这是 LLM Records 展示提示词、消息和
; 工具 schema 所需要的。
llmRecordMode = "full"
llmRecordRetentionDays = 30
llmRecordMaxRecords = 10000
llmRecordMaxPayloadBytes = 1048576
```

请求体在存储前会被脱敏：看起来像凭据的内容会被替换掉，替换次数会随记录一起显示。请求头（入站 API Key 就在里面）根本不会进入记录。超过 `llmRecordMaxPayloadBytes` 的请求体会保留结构、只截断其中最长的字符串，所以一段很长的对话仍然能逐条消息列出来。

每条记录旁边的费用用的是内置的官方标价，而厂商会调价、经销商也不按它来。把 `llmPricingFile` 指向你自己的费率 JSON 文件即可修正。

### 开启 WAF 反向代理

反向代理默认关闭，所以安装 Gateway 不会占用 80 和 443 端口。要使用它：

1. **Advanced → Sites → Add**。**Domain** 填客户端会使用的主机名（`test.example.com`），**Host** 和 **Port** 填流量的去向（`127.0.0.1` 和 `8000`），**Mode** 选 `HTTP` —— 默认的 `HTTPS Only` 会在请求到达后端之前把明文 HTTP 重定向走。
2. 打开 Sites 页面顶部的 **反向代理** 开关。它立即生效，重启后依然保持。Linux 和 macOS 上 80/443 端口需要 root，所以第一次尝试可以在 **设置** 页面里把网关 HTTP 端口改成 `8080`。
3. 在后端端口上起点什么，例如 `python -m http.server 8000`，然后用 `Host` 头请求这个站点 —— 网关就是按它路由的，所以不需要 DNS 或 `hosts` 记录：

```bash
curl -H "Host: test.example.com" http://127.0.0.1:8080/
```

你应该会拿到后端的响应。返回 `site not found for host` 说明请求到达了 Gateway，但没有站点匹配这个 `Host` 值。

### 接入 Casdoor

[Casdoor](https://casdoor.org) 是可选的，接管成员管理。在一个 Casdoor 实例里为 Gateway 创建组织和应用，然后在 **设置 → 登录** 里填好那五项。只要设置了 `casdoorEndpoint`，登录就会跳转到 Casdoor，同时还会启用 [OAuth 登录](https://casdoor.org/docs/provider/oauth/overview)、健康检查告警、`CAPTCHA` 规则动作、按站点的 SSO 以及云端文件存储。

## 开发

### 环境要求

Go 1.20+，以及带 Yarn 的 Node.js。

### 从源码运行

后端从 `web/build` 提供编译好的前端，所以先构建一次：

```bash
cd web && yarn install && yarn build
```

```bash
go run main.go
```

然后打开 http://localhost:17000，用 `admin` 和密码 `123` 登录，和安装版一样。SQLite 数据库在首次启动时创建，不需要安装数据库服务。

### 前端开发

```bash
cd web && yarn dev
```

这会在 http://localhost:16002 上提供带热更新的 UI，并把 API 请求代理到 17000 端口的后端，所以两边都得跑着。

### 使用 MySQL 代替 SQLite

项目用的是 XORM，所以它支持的每种数据库都能用。把 Gateway 指向你的服务器，首次启动时如果 `dbName` 不存在它会自动创建：

```ini
driverName = mysql
dataSourceName = root:123@tcp(localhost:3306)/
dbName = casbin_gateway
```

### 构建单文件二进制

Gateway 平时会从磁盘读三样东西：`conf/app.conf`、`web/build` 里编译好的 UI，以及 IP 地理位置库 `ip/17monipdb.dat`。`embed` 构建标签会把这三样都打进可执行文件，安装脚本发布的就是这种产物：

```bash
cd web && yarn install && yarn build
```

```bash
go build -tags embed -o casbin-gateway .
```

要先构建前端 —— `web/build` 下的所有内容都会进入二进制，该目录不存在时 `go build -tags embed` 会编译失败。

磁盘上的文件始终优先于内嵌副本，所以单文件二进制照样可以被配置和用于开发，不必重新构建：

| 内嵌资源 | 被什么覆盖 |
| --- | --- |
| `conf/app.conf` | 工作目录下、或可执行文件旁边的 `conf/app.conf` |
| `web/build` | 工作目录下的 `web/build/index.html`，此后整个 UI 都从那里提供 |
| `ip/17monipdb.dat` | 工作目录下的 `ip/17monipdb.dat` |

启动摘要会报告每一项实际来自哪里。

### 数据存放位置

自包含说的是启动，不代表运行时只读。运行中的 Gateway 会相对于工作目录写入 `./data`（SQLite 数据库、部署的应用、Agent 的 patch 状态）、`./logs` 和 `./tmp` —— 这也是为什么安装出来的 `casbin-gateway` 命令是一个总在安装目录里启动它的包装脚本。直接在别处运行可执行文件，会在那里得到第二个空的安装。

## 架构

Casbin Gateway 包含 2 个部分：

| 名称 | 说明 | 语言 | 源代码 |
|----------|----------------------------------------|------------------------|----------------------------------------------------------|
| 前端 | Casbin Gateway 的 Web 前端 UI | TypeScript + React + shadcn/ui | https://github.com/apache/casbin-gateway/tree/master/web |
| 后端 | Casbin Gateway 的 RESTful API 后端 | Golang + Beego + XORM | https://github.com/apache/casbin-gateway |

## 在线演示

https://ai.casbin.com

## 文档

https://caswaf.org

## 贡献

有任何问题欢迎提 issue，或者直接提 pull request —— 不过我们建议先开一个 issue 和社区讨论清楚。

## 许可证

[Apache-2.0](https://github.com/apache/casbin-gateway/blob/master/LICENSE)
