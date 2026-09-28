# CampusClaw · 第 3 课基础与第 4 课可追溯检索

按[第 3 课课件](https://devops.hello1023.com/%E8%AF%BE%E4%BB%B6/%E7%AC%AC3%E8%AF%BE-%E8%AF%BE%E4%BB%B6-%E8%AE%A4%E8%AF%81%E6%8E%88%E6%9D%83%E4%B8%8E%E7%9F%A5%E8%AF%86%E5%BA%93%E5%85%A5%E5%BA%93/index.html)实现的教学材料库，并在此基础上实现[第 4 课](https://devops.hello1023.com/%E8%AF%BE%E4%BB%B6/%E7%AC%AC4%E8%AF%BE-%E8%AF%BE%E4%BB%B6-%E5%8F%AF%E8%BF%BD%E6%BA%AF%E7%9F%A5%E8%AF%86%E5%BA%93%E6%A3%80%E7%B4%A2/index.html)的持久切片、三种检索与可追溯简短回答。前端为 React 18、TypeScript、Vite；Nginx 提供同源入口；Go `net/http` 提供 API；MySQL 8.0 保存账号、会话、原文和切片；Qdrant 保存向量。Docker Compose 是标准启动方式。

本课只接收 UTF-8 `.txt`、`.md`：A 班教师可以上传本班文件，A 班师生可以在列表查找文件名、查看提取正文并经鉴权下载原文件。学生上传返回 403；跨班材料 ID 和不存在的 ID 都返回相同的 404。列表来自数据库，没有前端硬编码材料。PDF/DOCX 解析、OCR、公开注册、SSO、作业批改、流式长对话与生产多副本不在本课范围。

## 第 4 课检索与回答

上传后原文保持不变，索引器按 `auto`（默认最大 800 字、重叠 80 字）、`custom`（分隔符/长度/重叠/可选预处理）或 `hierarchy`（Markdown 标题）生成切片。切片正文、序号、Unicode 字符范围和状态存于 MySQL `knowledge_chunks`；嵌入向量写入 Compose 内的 Qdrant，点 ID 等于切片 ID，payload 不含正文。已有材料启动时自动补建默认索引。教师可在材料详情按当前策略重建；嵌入失败时原文件与原文仍保留，页面显示索引失败。

`GET /api/retrieval?q=问题&mode=keyword|vector|hybrid` 默认 `hybrid`。关键词只查 MySQL ngram FULLTEXT；向量模式先嵌入问题，再按会话班级查询 Qdrant，余弦低于 0.35 的候选丢弃，回 MySQL 时再次核对班级；混合模式在两路过滤后以 RRF（k=60）排序。每条结果包含材料 ID、标题、切片序号、字符范围和取自 MySQL 的摘录，可打开原文。无结果返回空 `hits` 和「资料中未找到相关内容」。`POST /api/ask` 用混合检索取本班前四条，有依据才调用对话网关并返回编号出处；无依据不调用模型。关键词模式在 Qdrant 不可用时仍能使用，向量/混合/问答返回 503。匿名 401、空查询 400；客户端传入的班级或角色字段被忽略，授权始终使用会话班级。新增完整变更为 `openspec/changes/add-traceable-vector-retrieval/`，先前关键词 MVP 保留在 `add-traceable-keyword-retrieval/`。

## 从零启动

需要 Docker 与 Docker Compose。克隆仓库后在根目录执行：

```sh
cp .env.example .env
```

编辑 `.env`，把 `SESSION_SECRET`、`DB_PASSWORD`、`DB_ROOT_PASSWORD` 和三个 `SEED_*_PASSWORD` 的占位符全部换成各自独立的强随机值。另填课程网关的 `EMBEDDING_BASE_URL`、`EMBEDDING_MODEL`、`EMBEDDING_API_KEY` 及 `CHAT_BASE_URL`、`CHAT_MODEL`、`CHAT_API_KEY`。Base URL 应指向兼容 OpenAI 的 `/v1` 根路径，Go 服务会分别追加 `/embeddings` 与 `/chat/completions`。模型密钥只交给 API 容器，不进入浏览器或仓库。若暂未配置网关，材料仍可保存，但索引标记失败；配置后重启会自动重试，也可由教师手动重建。`.env` 已被 Git 与 Docker 构建上下文排除。可按需修改 `WEB_PORT`（默认 8080）和 `MAX_UPLOAD_BYTES`（默认 10485760）。

```sh
docker compose up --build -d
docker compose ps
curl http://localhost:8080/health
```

浏览器打开 `http://localhost:8080/login`。若改了 `WEB_PORT`，URL 中使用相应端口。全新 MySQL 初始化可能需要几十秒；Go API 会等待数据库可连接再迁移建表和运行幂等种子。`GET /health` 不需要登录，只表示 API 进程存活。数据库故障时它仍返回 `200 {"status":"ok"}`，受影响的材料 API 返回 503。

预置账号为 `teacher_a`（教师、A 班）、`student_a1`（学生、A 班）、`student_b1`（学生、B 班）。各自密码来自同名 `SEED_*_PASSWORD` 环境变量，没有仓库内共享默认密码。初始化还创建可区分的 A/B 班材料及正文；重复启动不会复制种子，也不会覆盖教师上传的材料。B 班初始材料标记为系统初始化，不代表学生获得上传权限。

登录后 `GET /api/me` 返回服务端读取的角色、班级和 CSRF 凭据。`POST /api/materials` 使用 `multipart/form-data` 的 `file` 字段和 `X-CSRF-Token` 请求头。浏览器页面会自动处理这两项。原文件只挂载到 API 容器，必须经 `/api/materials/{id}/file` 鉴权下载；`/uploads/...` 不公开。

## 验收与排查

设置好 `.env` 后，可以运行完整成功与失败路径：

```sh
set -a
. ./.env
set +a
python3 scripts/verify_flow.py
```

脚本需要可用的嵌入与对话网关，仅输出状态码、材料 ID 和列表数量，不打印密码或会话 Cookie。它验证教师上传、三种检索、可追溯出处、问答、无依据、教师重建、A/B 隔离、匿名 401、学生上传 403 与不支持扩展名 400。服务端还对空文件、非法 UTF-8 返回 400，对超限文件返回 413；数据库和磁盘写入失败时回滚记录并清理文件。可用 `scripts/mock_gateway.py` 在隔离测试环境模拟兼容 OpenAI 的接口；该脚本只验证链路，不提供真实语义检索能力。

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
