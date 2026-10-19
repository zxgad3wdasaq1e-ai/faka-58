# XIOEN AI 发卡商城

基于 [Dujiao-Next](https://github.com/dujiao-next/dujiao-next) 的发卡商城，参考 [GETGPT](https://getgpt.pro/) 公开页面重建店面。面向 `xioenai.vip` 部署，品牌、商品、价格及客服信息可在后台维护。

包含分类和 SKU、卡密批量导入与自动交付、人工交付、游客查单、会员、购物车、优惠券、库存预占、订单管理、支付配置、后台 RBAC 和 2FA。初始化目录不含真实卡密、虚构销量或用户评价，展示商品需要先导入库存或配置人工交付。

技术栈：Go 1.26.5 / Gin / GORM，Vue 3 / TypeScript / Tailwind CSS，PostgreSQL 17、Redis 7。Docker 多阶段构建将前台和后台嵌入同一二进制。默认图床为持久化 `uploads`，可选 MinIO。

## Ubuntu 一键部署

先安装 Git、Python 3.9+、Docker Engine 和 Docker Compose v2（支持 `up --wait`）。首次构建需要访问 npm、Go modules、镜像仓库，建议至少 2 核 / 4 GB 内存。将 `xioenai.vip` 的 DNS A/AAAA 指向服务器，开放入站 TCP 80、443；UDP 443 为可选 HTTP/3。

```bash
git clone https://github.com/zxgad3wdasaq1e-ai/faka-58.git
cd faka-58
python3 scripts/setup.py --domain xioenai.vip --start --seed
```

脚本生成三组独立应用密钥、数据库/Redis 密码、随机后台路径和首个管理员密码；启用 release 模式、关闭示例 SMTP；构建并启动 PostgreSQL、Redis、应用和 Caddy，等待健康检查后导入初始目录与品牌。Caddy 自动申请和续期 HTTPS 证书。脚本不会覆盖已有配置或自动轮换密钥。

店面地址：`https://xioenai.vip`。后台地址、用户名及首次密码仅写入服务器 `deploy/secrets/initial-admin.txt`，不在终端打印。首次登录后修改管理员密码并启用 2FA。`config.yml`、`.env`、`deploy/secrets/`、`deploy/runtime/` 和备份已同时排除在 Git 与镜像构建上下文之外；不要提交或公开这些文件。

只生成配置、不启动服务：

```bash
python3 scripts/setup.py --domain xioenai.vip
docker compose config --quiet
docker compose up -d --build --wait
python3 scripts/seed-catalog.py --base-url https://xioenai.vip
```

初始化脚本只依赖 Python 标准库，生成的 `config.yml` 为兼容 YAML 的 JSON 格式，可直接编辑。应用首次启动自动迁移数据库并创建管理员；商品导入为幂等操作，不导入支付密钥或真实卡密。配置真实 SMTP 后再启用邮件验证与通知。

本地体验：`python3 scripts/setup.py --local --start --seed`，地址 `http://localhost:8080`，不申请证书。Windows 使用 `python` 替换 `python3`。再次运行会沿用原域名、存储方式和密钥；修改前先备份，再调整 `.env`、`config.yml` 和 `deploy/runtime/Caddyfile`。网段冲突时，首次生成可指定 `--subnet 172.28.240.0/24`。

## 微信和支付宝：易支付兼容平台

默认不创建收款渠道。首次使用支付管理时，需要站点管理员在后台阅读并确认项目自带的运营声明；初始化脚本不会代为确认。登录后台「支付渠道」，按平台支持的协议添加微信、支付宝两个渠道，服务商选择「易支付」。支付方式代码、费率和币种以平台参数为准。

| 参数 | 配置 |
|---|---|
| 网关地址 | 平台提供的 HTTPS 网关 |
| 商户号 | 平台分配的商户 ID |
| v1 签名 | MD5 商户密钥 |
| v2 签名 | 商户私钥、平台公钥 |
| 异步通知 | `https://xioenai.vip/api/v1/payments/callback` |
| 支付后回跳 | `https://xioenai.vip/pay` |
| 支付交互 | 按网关选择跳转收银台或二维码 |

在平台完成 `xioenai.vip` 域名登记/白名单配置并允许回调。回跳页面展示订单状态；发货依据服务端验签及订单号、渠道、金额、币种核对后的结果。先导入一张专用测试卡，用平台允许的小额真实订单验证微信、支付宝各一次，并检查重复回调不会重复发卡。未提供商户参数时，只能验证界面、订单和库存，不能验收真实收款。

保留上游官方微信 API v3、支付宝 RSA2 等适配器，后续可在后台填写证书、公钥和私钥切换直连。真实配置不写入源码。项目不生成模拟付款成功入口。

## 图床和管理工具

默认 `/uploads/` 由应用提供，图片保存在 Docker `uploads` 卷，重建镜像不会丢失。类型、大小、尺寸限制由配置控制。

首次部署可选择 MinIO：

```bash
python3 scripts/setup.py --domain xioenai.vip --storage minio --start --seed
```

脚本通过 `.env` 合并 `compose.minio.yml` 并启用 `storage` profile。MinIO 从固定的[官方安全版本源码](https://github.com/minio/minio/releases/tag/RELEASE.2025-10-15T17-29-55Z)构建，不依赖旧的预构建镜像，首次构建较慢。初始化容器建立 `storefront-media` bucket、仅可写入/清理图片的应用账号和匿名对象读取策略。应用通过内网 S3 SigV4 上传，公开图片地址为 `https://xioenai.vip/media/...`，Caddy 仅开放该路径的 GET/HEAD。管理账号与应用账号分离。

MinIO 控制台仅绑定 `127.0.0.1:9001`，凭据位于 `deploy/secrets/initial-admin.txt`。使用 `ssh -L 9001:127.0.0.1:9001 user@服务器IP` 转发后访问。

可选 Adminer 数据库工具：

```bash
docker compose --profile tools up -d adminer
# SSH 转发后访问 http://127.0.0.1:8090
# 类型 PostgreSQL，服务器 postgres，数据库/用户名 getgpt
docker compose --profile tools stop adminer
```

应用数据库账号 `getgpt` 是数据库所有者，不具有 PostgreSQL 超级用户、建库或建角色权限；应用密码位于 `deploy/secrets/postgres_password`，管理员密码另存为 `postgres_admin_password`。PostgreSQL、Redis 和 MinIO S3 端口均不向宿主机公开；管理工具仅监听回环地址。

## 运维、备份和恢复

```bash
docker compose ps
docker compose logs --tail 100 app
docker compose logs --tail 100 caddy
curl --fail https://xioenai.vip/health
```

Caddy 是应用唯一受信任的代理地址。增加 CDN 或更改网络时，应同步设置真实可信代理链。更新前先备份，再执行 `git pull --ff-only`、`docker compose up -d --build --wait`；容器部署通过重建镜像更新。

备份会短暂停止店面、队列及对象存储，结束后恢复原先运行的服务：

```bash
python3 deploy/backup.py backup
```

输出到 `backups/UTC时间/`，包含 PostgreSQL 一致性 dump、uploads、Redis、MinIO（已启用时）、Caddy、应用状态和完整私有配置，以及 SHA-256 校验清单。备份包含支付配置及密钥，必须加密保存到异机。恢复必须保留同一 `app.secret_key`，否则不能解密敏感配置。

在新主机、全新 checkout 和空数据卷中恢复，先安全传入备份目录，不要先运行 setup：

```bash
git clone https://github.com/zxgad3wdasaq1e-ai/faka-58.git
cd faka-58
python3 deploy/backup.py restore --source /安全路径/备份目录 --confirm-replace-database
```

恢复工具先校验备份并预检全部目标文件卷，拒绝覆盖已有配置和非空文件卷，再恢复密钥、文件卷及数据库。数据库导入为单个事务；恢复进度写入私有目录，网络或磁盘故障修复后重新执行同一条恢复命令可继续，不能混用另一份备份。该确认参数明确允许替换目标 PostgreSQL 数据。恢复后核对商品、历史订单、图片和回调。不要使用 `docker compose down -v` 重启，它会删除持久化卷。

## 本地开发和构建

使用 Go 1.26.5、pnpm 10.34.3。复制 `config.yml.example` 到 `config.yml` 并填写三个不同强密钥。本地可使用 SQLite 与独立 Redis；setup 生成的是 Compose 内部数据库地址，宿主机运行需修改连接参数。

```bash
go run ./cmd/server
# 分别在两个终端进入前台、后台目录执行
cd frontend/user
corepack enable
pnpm install --frozen-lockfile
pnpm run dev
# 后台同理：frontend/admin
```

前台端口 5173，后台 5174，Vite 代理 API/上传至后端 8080。镜像构建自动构建两前端并使用 `release,fullstack` 标签嵌入：

```bash
docker build -t getgpt-store:local .
go vet ./...
go test ./...
# 两前端目录各执行 pnpm run test 和 pnpm run build
```

生产 DNS/TLS 和真实支付须在目标服务器验收。当前开发环境缺少 Docker CLI，尚未在此环境执行镜像构建、Compose 启动、MinIO 容器和真实收款验证；上述命令为目标 Ubuntu 主机的构建与验收入口。

本地已通过两前端共 90 项测试、类型检查及生产构建、`go vet ./...`、Windows 和 Linux amd64 完整程序构建，以及隔离测试库中的微信/支付宝易支付模拟回调、错误签名/金额/商户号拒绝、自动发卡、重复回调和图片上传删除验证。Windows 的完整 Go 测试剩 7 项自更新测试失败，依赖 Linux/macOS 的 `flock`、POSIX 权限或操作系统探测前提；业务模块测试通过，完整 Linux 测试由 Ubuntu CI 验证。模拟测试没有真实收款，不能代替商户平台联调。

## 来源和许可证

本项目保留 [Dujiao-Next](https://github.com/dujiao-next/dujiao-next) 的业务模块、后台、支付和库存实现，修改版继续采用 [GNU GPL v3](LICENSE)。公开分发源码或镜像时保留版权、许可证及对应源码。`getgpt.pro` 为公开界面参考，不包含其私有服务端源码，不与其运营方关联。商品、经营主体、品牌素材、售后条款由部署者维护。

MinIO 和 mc 是独立可选服务，其镜像附带上游 LICENSE；其他依赖和商标保留各自权利。
