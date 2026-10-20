# XIOEN AI 发卡商城

基于 [Dujiao-Next](https://github.com/dujiao-next/dujiao-next) 的发卡商城，参考 [GETGPT](https://getgpt.pro/) 公开页面重建店面。面向 `xioenai.vip` 部署，品牌、商品、价格及客服信息可在后台维护。

包含分类和 SKU、卡密批量导入与自动交付、人工交付、游客查单、会员、购物车、优惠券、库存预占、订单管理、支付配置、后台 RBAC 和 2FA。初始化目录不含真实卡密、虚构销量或用户评价，展示商品需要先导入库存或配置人工交付。

技术栈：Go 1.26.5 / Gin / GORM，Vue 3 / TypeScript / Tailwind CSS，PostgreSQL 17、Redis 7。Docker 多阶段构建将前台和后台嵌入同一二进制。默认图床为持久化 `uploads`，可选 MinIO。

首页还原了首屏短句轮播、双排反向连续滚动评价和吸顶导航。登录后台「系统设置 → 首页内容」可编辑三语短句、轮换间隔、评价内容及滚动速度；保存后刷新前台生效。评价墙默认使用标有演示说明的原创模拟内容，可替换或关闭。动画支持悬停暂停、手动暂停和系统减少动态效果偏好。

同一设置页可配置「首屏随机播报」的开关、间隔、昵称列表与三语消息；`{name}` 替换为随机脱敏昵称，`{count}` 为 3～5 的随机数。播报按打乱顺序循环，支持暂停；此展示组件不读取实际订单或在线人数。

## Ubuntu 快速部署（推荐：预构建镜像）

GitHub 负责保存代码、测试和构建镜像，网站运行在你的 Ubuntu 服务器上。服务器只需 Docker Engine、Docker Compose v2（支持 `up --wait`）、Shell 和 curl；无需安装 Python、Node.js、Go 或 jq，也不在服务器编译前后台。将 `xioenai.vip` 的 DNS A/AAAA 指向服务器，开放 TCP 80、443；已有服务占用这些端口时需先协调反向代理。

首次发布需要仓库管理员完成一次：将部署改动上传到 `main`，等待 Actions 中 `ci` 和 `Publish Docker image` 成功；在 GitHub 个人主页的 Packages 中打开 `faka-58` → Package settings → Change visibility → Public。公开源码仓库不会自动将容器包设为公开。流水线使用仓库自带的 `GITHUB_TOKEN`，无需把个人令牌写进代码。

镜像发布后，在服务器的空目录中运行：

```bash
mkdir -p ~/faka-58 && cd ~/faka-58
curl -fsSL https://raw.githubusercontent.com/zxgad3wdasaq1e-ai/faka-58/main/deploy/docker-deploy.sh | bash -s -- --domain xioenai.vip
```

Shell 脚本拉取 `ghcr.io/zxgad3wdasaq1e-ai/faka-58:latest`，校验镜像来源及源码版本，下载同版本部署文件并固定到镜像 SHA256 摘要；不会下载整个源码仓库。镜像内的 Go 工具 `storectl` 使用系统安全随机源生成独立密钥和管理员账号，然后启动 PostgreSQL、Redis、店面和 Caddy，申请 HTTPS 并幂等导入初始商品目录。图片使用持久化 `uploads` 卷，自动提供图床，无需额外安装语言运行时。服务器同时支持 amd64 和 arm64；全新安装脚本拒绝覆盖非空目录和既有密钥。

店面：`https://xioenai.vip`。在部署目录读取首次后台地址与账号：

```bash
cat deploy/secrets/initial-admin.txt
docker compose ps
docker compose logs --tail 100 app
```

只通过 HTTP 体验时将 `--domain xioenai.vip` 换为 `--local`，在服务器本机访问 `http://localhost:8080`。远程体验可运行 `ssh -L 8080:127.0.0.1:8080 用户名@服务器IP` 后在自己电脑打开同一地址。正式收款前仍需在后台配置易支付、导入卡密或设置人工交付，并完成真实订单测试。

拉取镜像报 `denied` 时，先检查镜像发布是否成功及 Package 是否已设为 Public；私有镜像需在服务器自行 `docker login ghcr.io`，不要将令牌发给他人。配置生成完成但服务启动或目录初始化失败后，在原目录沿用配置重试：

```bash
sh deploy/start.sh --seed
```

镜像方式使用 `docker compose pull` 和 `up --no-build`；既有源码部署不会被自动切换。快速安装固定版本以便复现，日常重启使用 `docker compose up -d --no-build --wait`；单独执行 `pull` 不会将已固定摘要升级到新版本。升级前先备份，明确选定新镜像版本后同步调整 `.env` 的 `APP_IMAGE` 与 `deploy/runtime/installation.json` 的 `image`；跨版本部署文件应取自对应源码版本，不要混用。备份及空环境恢复方法见下文。

## Ubuntu 源码部署（兼容旧工具）

以下 Python 工具保留给既有源码部署与可选 MinIO 源码构建；它们不参与上面的无 Python 一键安装，新用户直接使用快速部署即可。旧配置和密钥不会被新流程自动切换或覆盖。

先安装 Git、Python 3.8+、Docker Engine 和 Docker Compose v2（支持 `up --wait`）。首次构建需要访问 npm、Go modules、镜像仓库，建议至少 2 核 / 4 GB 内存。将 `xioenai.vip` 的 DNS A/AAAA 指向服务器，开放入站 TCP 80、443；UDP 443 为可选 HTTP/3。

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

Caddy 是应用唯一受信任的代理地址。增加 CDN 或更改网络时，应同步设置真实可信代理链。更新前先备份。源码部署执行 `git pull --ff-only`、`docker compose up -d --build --wait`；镜像安装按上文选择对应版本，使用 `up --no-build` 启动。

无 Python 快速安装使用 Shell 备份工具。备份会短暂停止店面和队列，结束或失败后恢复原先运行的服务：

```bash
sh deploy/backup.sh backup
```

默认输出到 `backups/UTC时间/`，包含 PostgreSQL 一致性 dump、uploads、Redis、Caddy、应用状态、应用日志和完整私有配置，以及 `SHA256SUMS` 校验清单。备份包含支付配置及密钥，必须加密保存到异机。恢复必须保留同一 `app.secret_key`，否则不能解密敏感配置。

在新主机、具有同版本公开部署文件的空安装目录和空数据卷中恢复，先安全传入备份目录，不要先运行初始化。可在另一台电脑准备同版本源码或在服务器克隆公开文件；恢复本身无需 Python：

```bash
git clone https://github.com/zxgad3wdasaq1e-ai/faka-58.git
cd faka-58
sh deploy/backup.sh restore --source /安全路径/备份目录 --confirm-replace-database
```

恢复工具先校验备份格式、摘要及归档路径，再预检目标配置、容器和数据卷；已有私有配置、同项目容器或非空卷都会被拒绝。数据库导入为单个事务。Shell 恢复不覆盖已有安装，也不自动清理失败现场；中途失败应保留现场排查，不能直接重跑或删除密钥。恢复后核对商品、历史订单、图片和回调。不要使用 `docker compose down -v` 重启，它会删除持久化卷。

Shell 工具仅处理新快速安装的持久化本地图床与预构建镜像，备份格式为 Shell v1。旧 Python 备份及 MinIO/源码安装继续使用原 `python3 deploy/backup.py backup` / `restore` 流程，两种备份格式不可混用。

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

本地已通过两前端共 104 项测试、类型检查及生产构建、`go vet ./...`、Windows 和 Linux amd64 完整程序构建，以及隔离测试库中的微信/支付宝易支付模拟回调、错误签名/金额/商户号拒绝、自动发卡、重复回调和图片上传删除验证。Windows 的完整 Go 测试仍有自更新测试因平台前提失败，依赖 Linux/macOS 的 `flock`、POSIX 权限或操作系统探测前提；业务模块测试通过，完整 Linux 测试由 Ubuntu CI 验证。模拟测试没有真实收款，不能代替商户平台联调。

## 来源和许可证

本项目保留 [Dujiao-Next](https://github.com/dujiao-next/dujiao-next) 的业务模块、后台、支付和库存实现，修改版继续采用 [GNU GPL v3](LICENSE)。公开分发源码或镜像时保留版权、许可证及对应源码。`getgpt.pro` 为公开界面参考，不包含其私有服务端源码，不与其运营方关联。商品、经营主体、品牌素材、售后条款由部署者维护。

MinIO 和 mc 是独立可选服务，其镜像附带上游 LICENSE；其他依赖和商标保留各自权利。

首页使用的 Inter 字体来自 [rsms/inter](https://github.com/rsms/inter)，未修改字体文件；字体继续使用随附的 SIL Open Font License，见 `frontend/user/public/fonts/inter/LICENSE.txt`。
