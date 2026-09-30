# CampusClaw · 第 3 课基础与第 4 课可追溯检索

按[第 3 课课件](https://devops.hello1023.com/%E8%AF%BE%E4%BB%B6/%E7%AC%AC3%E8%AF%BE-%E8%AF%BE%E4%BB%B6-%E8%AE%A4%E8%AF%81%E6%8E%88%E6%9D%83%E4%B8%8E%E7%9F%A5%E8%AF%86%E5%BA%93%E5%85%A5%E5%BA%93/index.html)实现的教学材料库，并在此基础上实现[第 4 课](https://devops.hello1023.com/%E8%AF%BE%E4%BB%B6/%E7%AC%AC4%E8%AF%BE-%E8%AF%BE%E4%BB%B6-%E5%8F%AF%E8%BF%BD%E6%BA%AF%E7%9F%A5%E8%AF%86%E5%BA%93%E6%A3%80%E7%B4%A2/index.html)的持久切片、三种检索与可追溯简短回答。前端为 React 18、TypeScript、Vite；Nginx 提供同源入口；Go `net/http` 提供 API；MySQL 8.0 保存账号、会话、原文和切片；Qdrant 保存向量。Docker Compose 是标准启动方式。

本课只接收 UTF-8 `.txt`、`.md`：A 班教师可以上传本班文件，A 班师生可以在列表查找文件名、查看提取正文并经鉴权下载原文件。学生上传返回 403；跨班材料 ID 和不存在的 ID 都返回相同的 404。列表来自数据库，没有前端硬编码材料。PDF/DOCX 解析、OCR、公开注册、SSO、作业批改、流式长对话与生产多副本不在本课范围。

## 第 4 课检索与回答

上传后原文保持不变，索引器按 `auto`（默认最大 800 字、重叠 80 字）、`custom`（分隔符/长度/重叠/可选预处理）或 `hierarchy`（Markdown 标题）生成切片。切片正文、序号、Unicode 字符范围和状态存于 MySQL `knowledge_chunks`；嵌入向量写入 Compose 内的 Qdrant，点 ID 等于切片 ID，payload 不含正文。已有材料启动时自动补建默认索引。教师可在材料详情按当前策略重建；嵌入失败时原文件与原文仍保留，页面显示索引失败。

`GET /api/retrieval?q=问题&mode=keyword|vector|hybrid` 在嵌入模型已配置时默认 `hybrid`，否则默认 `keyword`。关键词只查 MySQL ngram FULLTEXT；向量模式先嵌入问题，再按会话班级查询 Qdrant，余弦低于 0.35 的候选丢弃，回 MySQL 时再次核对班级；混合模式在两路过滤后以 RRF（k=60）排序。每条结果包含材料 ID、标题、切片序号、字符范围和取自 MySQL 的摘录，可打开原文。无结果返回空 `hits` 和「资料中未找到相关内容」。`POST /api/ask` 用混合检索取本班前四条，有依据才调用对话网关并返回编号出处；无依据不调用模型。关键词模式在 Qdrant 不可用或嵌入模型未配置时仍能使用，向量/混合/问答返回 503。匿名 401、空查询 400；客户端传入的班级或角色字段被忽略，授权始终使用会话班级。新增完整变更为 `openspec/changes/add-traceable-vector-retrieval/`，先前关键词 MVP 保留在 `add-traceable-keyword-retrieval/`。

## 从零启动

需要 Docker 与 Docker Compose。克隆仓库后在根目录执行：

```sh
cp .env.example .env
```

编辑 `.env`，把 `SESSION_SECRET`、`DB_PASSWORD`、`DB_ROOT_PASSWORD` 和三个 `SEED_*_PASSWORD` 的占位符全部换成各自独立的强随机值。若先只用关键词检索，六个模型网关变量保持空白即可：上传和教师重建仍写入 MySQL 切片，页面默认关键词检索。数据库的 `failed` 状态此时仅表示**向量尚未生成**，不妨碍关键词检索。`GET /api/retrieval/capabilities` 可查看当前可用能力。准备开启完整功能时再填 `EMBEDDING_BASE_URL`、`EMBEDDING_MODEL`、`EMBEDDING_API_KEY` 及 `CHAT_BASE_URL`、`CHAT_MODEL`、`CHAT_API_KEY`。Base URL 应是兼容 OpenAI 的 API 根路径，Go 服务会分别追加 `/embeddings` 与 `/chat/completions`。模型密钥只交给 API 容器，不进入浏览器或仓库。配置嵌入模型后重启会自动补齐缺失向量，也可由教师手动重建。`.env` 已被 Git 与 Docker 构建上下文排除。可按需修改 `WEB_PORT`（默认 8080）和 `MAX_UPLOAD_BYTES`（默认 10485760）。

只开启向量和混合检索时，无需配置对话模型。登录[课程网关](https://ai-gateway.devops.hello1023.com/)取得本人 API Key 和可用的向量模型名，在本地 `.env` 填入以下三项（模型名以本人网关页面为准）：

```dotenv
EMBEDDING_BASE_URL=https://ai-gateway.devops.hello1023.com/v1
EMBEDDING_MODEL=course-embedding
EMBEDDING_API_KEY=在本地填入本人网关密钥
```

重建 API 容器后，启动时会自动重试此前标记为 `failed` 的切片；首次补建可能需要一段时间。材料列表状态变为 `ready` 后，可运行 `python3 scripts/verify_vector_only.py` 验证三种检索、本班可见和跨班隔离。切换嵌入模型时，必须确认它与现有 Qdrant 集合的向量维度一致；若不一致，应先按迁移方案重建向量集合，不能直接混用。

若要启用“简短回答”，同一课程网关还提供 `POST /v1/chat/completions`。在本地 `.env` 增加 `CHAT_BASE_URL=https://ai-gateway.devops.hello1023.com/v1`、`CHAT_MODEL=course-chat`，并将本人网关 Key 填入 `CHAT_API_KEY`，随后重建 API 容器。用于实际试用的六份演示材料和问题见[检索与问答试用材料](docs/retrieval-demo.md)。

```sh
docker compose up --build -d
docker compose ps
curl http://localhost:8080/health
```

浏览器打开 `http://localhost:8080/login`。若改了 `WEB_PORT`，URL 中使用相应端口。全新 MySQL 初始化可能需要几十秒；Go API 会等待数据库可连接再迁移建表和运行幂等种子。`GET /health` 不需要登录，只表示 API 进程存活。数据库故障时它仍返回 `200 {"status":"ok"}`，受影响的材料 API 返回 503。

预置账号为 `teacher_a`（教师、A 班）、`student_a1`（学生、A 班）、`student_b1`（学生、B 班）。各自密码来自同名 `SEED_*_PASSWORD` 环境变量，没有仓库内共享默认密码。初始化还创建可区分的 A/B 班材料及正文；重复启动不会复制种子，也不会覆盖教师上传的材料。B 班初始材料标记为系统初始化，不代表学生获得上传权限。

登录后 `GET /api/me` 返回服务端读取的角色、班级和 CSRF 凭据。`POST /api/materials` 使用 `multipart/form-data` 的 `file` 字段和 `X-CSRF-Token` 请求头。浏览器页面会自动处理这两项。原文件只挂载到 API 容器，必须经 `/api/materials/{id}/file` 鉴权下载；`/uploads/...` 不公开。

## 09/30 课堂任务：token 登录认证

浏览器现使用 `POST /api/login`。登录成功返回随机 `access_token`、`token_type: Bearer` 和到期时间，不设置登录 Cookie；前端将 token 保存在当前标签页的 `sessionStorage`，后续请求（含原文件下载）都带 `Authorization: Bearer <token>`。服务端数据库只保存 token 的带密钥哈希，每次请求仍从会话和用户表读取角色与班级。登出会立即删除会话；缺少、伪造、过期或已登出的 token 返回 401。Cookie 不能作为登录凭据。它是随机不透明 token，课程和作业截图未要求 JWT。

打开浏览器开发者工具的「网络」页，登录后点开 `/api/me`：请求头应有 `Authorization: Bearer …`、响应状态为 200，页面显示账号与班级。登出后再次用旧 token 请求 `/api/me` 应返回 401。**提交截图时遮住 token 原文和任何口令**。在 `.env` 准备好预置账号密码后，自动验证可运行 `python3 scripts/verify_token_auth.py`；脚本只输出状态码，不打印 token。

## 验收与排查

设置好 `.env` 后，可以运行完整成功与失败路径：

```sh
set -a
. ./.env
set +a
python3 scripts/verify_flow.py
```

脚本需要可用的嵌入与对话网关，仅输出状态码、材料 ID 和列表数量，不打印密码或会话 Cookie。它验证教师上传、三种检索、可追溯出处、问答、无依据、教师重建、A/B 隔离、匿名 401、学生上传 403 与不支持扩展名 400。服务端还对空文件、非法 UTF-8 返回 400，对超限文件返回 413；数据库和磁盘写入失败时回滚记录并清理文件。可用 `scripts/mock_gateway.py` 在隔离测试环境模拟兼容 OpenAI 的接口；该脚本只验证链路，不提供真实语义检索能力。

仅使用关键词时，可在没有网关参数的环境运行 `python3 scripts/verify_keyword_only.py`；它验证默认关键词、上传、重建、跨班隔离，以及向量/问答明确返回 503。

遇到启动问题先执行 `docker compose ps` 与 `docker compose logs --tail=100 api db web`。缺少必需环境变量会在 Compose 配置或 API 启动时失败；错误只指明变量名。若端口已被占用，在 `.env` 调整 `WEB_PORT`。数据库与上传文件都使用具名卷，不要用容器内临时目录替代。

前端开发时可先按上文启动 Compose，再运行：

```sh
cd frontend
npm ci
npm run dev
```

Vite 将同源 `/api` 和 `/health` 代理到 `http://localhost:8080`；若 Compose 使用别的端口，运行前设置 `VITE_API_PROXY_TARGET=http://localhost:你的端口`。前端不直连 MySQL 或 API 容器端口。

## 持久化、备份与回滚

普通停止和重建不删卷：

```sh
docker compose down
docker compose up --build -d
```

已验证此过程保留账号、上传原文件与知识库正文。不要用 `docker compose down -v` 回滚；它会删除数据库和上传卷。升级前备份 MySQL 和上传目录，两个备份应对应同一时点：

```sh
docker compose exec -T db sh -c 'exec mysqldump -u root -p"$MYSQL_ROOT_PASSWORD" "$MYSQL_DATABASE"' > campusclaw.sql
docker compose exec -T api tar -C /data -cf - uploads > campusclaw-uploads.tar
```

若要回滚代码或镜像，恢复先前版本后运行 `docker compose up --build -d`，保留具名卷。若必须恢复数据，应在停止写入后同时恢复数据库备份与文件备份。当前登录限流器存于单个 Go 进程内，因此本课明确只运行一个 API 实例；多副本需要共享限流状态，超出本课范围。
