# 第 3 课验收记录

日期：2026-09-24。实现范围以 `openspec/changes/add-class-auth-knowledge-base/` 的四份 spec 为准。下列测试使用一次性 MySQL 8.0 数据库与临时随机口令；记录不含口令和 Cookie。`scripts/verify_flow.py` 在干净拷贝的 Compose 环境执行，得到：

```json
{"health":200,"anonymous":401,"student_upload":403,"cross_class":404,"invalid_extension":400,"uploaded_id":3,"a_list_count":2,"b_list_count":1}
```

同一隔离测试对材料 `3` 验证：A 班学生详情 200、原文件下载 200、正文与上传文本相同；B 班学生详情和下载均为 404。材料集成测试在被拒上传前后断言 A/B 列表仍为 2/1、知识库正文仍为 3 条、上传目录仍为 3 个文件。MySQL 实测拒绝空 `class_id`（1048）、错误角色（3819）以及上传者/正文班级与材料不一致（1452）。

## user-auth/spec.md

| Scenario | 结果 | 证据 |
| --- | --- | --- |
| Both roles can log in | 通过 | `scripts/verify_flow.py`，三个账号均登录并由 `/api/me` 得到角色与班级。 |
| Invalid credentials | 通过 | `backend/internal/auth/service_integration_test.go`：不存在账号与错误密码均为 401 且响应体相同。 |
| Rate-limited credentials | 通过 | 同一测试：达到阈值后正确口令仍返回同形 401。 |
| Provision teacher and student | 通过 | `backend/internal/db/seed_integration_test.go` 与干净 Compose 验收：预置账号从环境变量取密码并登录。 |
| Unprovisioned account | 通过 | 认证集成测试中未知账号为 401；运行中 `POST /api/users` 返回 404。 |
| Two-class acceptance accounts | 通过 | `scripts/verify_flow.py`：教师 A、学生 A1 属 A；学生 B1 属 B。 |
| Idempotent seed | 通过 | `backend/internal/db/seed_integration_test.go`：两次执行后 2 班、3 用户、2 种子材料，额外教师材料与正文保留。 |
| Anonymous page navigation | 通过 | In-app browser 从 `/materials` 匿名打开后地址变为 `/login`，页面无材料正文。 |
| Invalid API session | 通过 | 认证集成测试：伪造、过期及登出后的旧 Cookie 请求 `/api/me` 均为 401。 |
| Password storage verification | 通过 | 种子集成测试：两个相同测试密码得到不同 bcrypt 哈希且各自可校验；密钥扫描无明文。 |
| Logout invalidates session | 通过 | 认证集成测试：登出需 CSRF，成功后旧 Cookie 重放为 401。 |
| Forged cross-site upload | 通过 | 材料集成测试及 `scripts/verify_flow.py`：有会话但无 CSRF 上传为 403，记录与文件数不变。 |

## class-access-control/spec.md

| Scenario | 结果 | 证据 |
| --- | --- | --- |
| Forged class or role | 通过 | `backend/internal/materials/service_integration_test.go`：教师传 `class_id=2` 被拒；学生发 `X-Role: teacher` 仍为 403。 |
| Student calls endpoint directly | 通过 | 同一测试及 Compose 验收：携有效 CSRF 的学生上传为 403。 |
| Teacher uploads to own class | 通过 | Compose 上传材料 `3` 后只出现在 A 班列表；数据库复合外键约束上传者与材料同班。 |
| Student cannot gain teacher role | 通过 | 材料集成测试中伪造教师请求头仍为 403，未产生文件与记录。 |
| Cross-class resource access | 通过 | A 班请求 B 班详情和下载，与不存在 ID 的 404 响应体一致。 |
| Cross-class upload target | 通过 | 教师上传带 `class_id=2` 返回 403，两班列表数量不变。 |
| Filtered lists and knowledge entries | 通过 | 材料集成测试：A 班用 B 班标题筛选得到 `total=0`，B 班正文 ID 请求为 404。 |
| Two-class acceptance materials | 通过 | 种子 A/B 文件名可区分，Compose 验收 A/B 列表分别仅有本班材料。 |
| Same class read access | 通过 | Compose 验收：A 班学生读取教师新材料详情及文件均为 200。 |

## teaching-knowledge-base/spec.md

| Scenario | 结果 | 证据 |
| --- | --- | --- |
| Accepted upload appears in class list | 通过 | Compose 验收上传得材料 ID `3`，A 班学生列表、详情与下载可用。 |
| Persisted knowledge base record | 通过 | 材料集成测试检查材料与正文关联、班级一致及原文件内容一致。 |
| Empty material list | 通过 | 材料集成测试清空 B 班测试材料后，B 班列表为空。 |
| Rejected file | 通过 | 材料集成测试：空文件与 PDF 为 400，超限为 413 且请求体未被读完；无残留。 |
| Invalid text or persistence failure | 通过 | 材料集成测试：非法 UTF-8 为 400；注入知识库写入失败与磁盘失败均无材料、正文或文件残留。 |
| Read own class material | 通过 | Compose 验收 A 班学生详情与下载均为 200，正文与原文件一致。 |
| Safe Markdown display | 通过 | `npm run test:markdown`：标题正常渲染，原始 HTML 与 `javascript:` 链接不执行。 |

## application-runtime/spec.md

| Scenario | 结果 | 证据 |
| --- | --- | --- |
| Fresh deployment | 通过 | 排除 `.git`、`node_modules`、`dist` 后复制到干净目录；`docker compose up --build -d` 与 `scripts/verify_flow.py` 通过。 |
| Required seed credentials | 通过 | `docker compose --env-file /dev/null config --quiet` 失败，列出缺失变量名；配置单元测试拒绝缺失值。 |
| Restart persistence | 通过 | 不带 `-v` 执行 `down`/`up` 后，账号登录与材料 `3` 正文/文件读取均为 200。 |
| Rebuild containers without deleting volumes | 通过 | `up --build -d` 重建后，健康检查 200，原材料 `3` 仍可下载。 |
| Secret not exposed | 通过 | 对源码、前端构建、镜像元数据、容器日志与 `/health` 扫描临时随机密钥，五处均未发现。 |
| Missing required secret | 通过 | Compose 空环境配置退出码 1，只输出缺失配置名；`backend/internal/config/config_test.go` 验证错误不回显密钥值。 |
| Healthy application | 通过 | Nginx 入口 `GET /health` 返回 `200 {"status":"ok"}`。 |
| Database unavailable | 通过 | 停 db 后 `/health` 为 200、有效会话材料 API 为 503；启动 db 后恢复 200。 |
| Upload storage unavailable | 通过 | 临时禁止 API 访问上传卷时 `/health` 200、教师上传 503；恢复权限后正常。 |

补充运行检查：猜测 `/uploads/guessed-file` 返回 Nginx 404；Vite 开发代理与 Compose 入口的匿名 `/api/me` 均返回 JSON 401。`docker compose config` 仅 web 映射宿主端口，上传卷只挂载 api。`npm run test:ui` 检查学生页面没有上传控件，教师页面有。`go test ./...`、`go build`、`npm run build` 和 OpenSpec 严格校验均通过。

## 第 4 课早期关键词检索 MVP（历史记录）

`scripts/verify_flow.py` 在运行中的 Compose 环境上传 A 班测试材料后，用 A 班学生检索正文得到带材料 ID、标题、摘录和字符范围的命中；B 班学生检索同一 A 班专属词返回空数组。匿名检索为 401，客户端指定 `class_id=2` 为 403。Go 单元测试验证中文字数位置、大小写匹配、字面 `%` 与结果上限；`npm run test:ui` 验证师生页面都有检索入口。无向量库或生成式回答，边界见 `openspec/changes/add-traceable-keyword-retrieval/`。

## 第 4 课完整 MVP 验收（2026-09-28）

在独立的 `cc-week04-check` Compose 项目及端口 `18083` 验证，未使用原有的 `18082` 应用和数据卷。使用 `scripts/mock_gateway.py` 模拟 OpenAI 兼容的嵌入和对话端点；它验证接口、索引与权限链路，不证明真实模型的语义质量。

- `go test ./...`、`npm run test:ui`、`npm run test:markdown`、`npm run build` 通过；`openspec validate add-traceable-vector-retrieval --strict` 通过。
- `scripts/verify_flow.py` 通过：教师上传后索引 `ready`，A 班学生的 keyword/vector/hybrid 均能命中并看到材料标题、切片号、Unicode 范围、摘录；B 班列表、详情、下载、检索均不能看到 A 班材料。客户端伪造 `class_id=2` 不改变会话班级。
- 有证据的 `/api/ask` 返回编号引用；无证据时返回「资料中未找到相关内容」与空引用，模拟网关计数确认未调用对话模型。教师可用 hierarchy 重建索引，学生重建 403。
- 停止隔离 Qdrant 后，keyword 为 200，vector/hybrid 为 503；重启后恢复。
- 将隔离数据库中的一条 custom 切片标为 failed 再重启 API，后台重试将同一切片 ID `8` 从 failed 改为 ready，`strategy=custom` 保持不变；已有种子材料在启动时自动补建切片。

真实课程网关的地址、模型名和密钥尚待本地配置，因此真实语义相关性和实际模型回答仍需配置后复验。密钥只放在本地 `.env`，不进入提交。
