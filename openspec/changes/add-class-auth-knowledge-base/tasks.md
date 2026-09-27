# Tasks

## 1. 骨架与配置

- [x] 1.1 建立 `backend/cmd/server`、`backend/internal/{config,db,auth,materials,knowledge}` 与 Go 模块；Verify：在干净环境执行 `go test ./...` 并编译服务入口。
- [x] 1.2 建立 React 18、TypeScript、Vite 的 `frontend/` 工程，预留登录与材料页面；Verify：依赖安装与生产构建成功。
- [x] 1.3 实现环境变量配置校验，提交仅含占位符的 `.env.example`、`.gitignore`、`.dockerignore`；Verify：缺少 SESSION_SECRET 或数据库凭据时启动失败，构建上下文与仓库不含真实密钥或上传文件。

## 2. 数据与幂等种子

- [x] 2.1 建立 MySQL 8.0 的 classes、users、sessions、materials、knowledge_entries 表；Verify：迁移检查 class_id 非空和索引、角色约束、外键及正文与材料班级一致性。
- [x] 2.2 实现从环境变量读取口令的幂等种子流程，预置 A/B 两班、teacher_a、student_a1、student_b1 及两班可区分材料与正文；Verify：连续执行两次不复制数据、不覆盖已上传材料，口令仅以 bcrypt 哈希存储。

## 3. 登录与会话

- [x] 3.1 实现 POST /api/login、POST /api/logout、GET /api/me 与 MySQL 服务端会话，记录接口与错误码；Verify：两类角色登录成功，登录换发会话 ID，旧会话及登出重放返回 401。
- [x] 3.2 实现 HttpOnly、SameSite=Lax Cookie、会话过期和每次请求从用户表读取角色/班级；Verify：伪造或过期 Cookie 返回 401，客户端伪造角色或班级无效，本地 HTTP Cookie 可用。
- [x] 3.3 实现同形登录错误、等价耗时比较、按用户名与 IP 限流及 Cookie 写操作 CSRF 防护；Verify：账号不存在、密码错误和限流响应一致，缺失 CSRF 的上传被拒且无写入。
- [x] 3.4 记录认证与会话行为；Verify：按文档重放匿名 API、错误口令、登出后旧 Cookie 请求并得到规定状态码。

## 4. 班级与角色隔离

- [x] 4.1 建立 Go 服务端身份上下文与教师上传权限；Verify：学生直接上传返回 403，声明自己是教师或指定 B 班均不能写入。
- [x] 4.2 实现列表/计数按会话班级查询，详情/正文/文件按 ID 读取后核验班级并统一返回 404；Verify：A/B 两班各自只读本班材料，跨班 ID 与不存在 ID 响应一致且无内容泄露。
- [x] 4.3 确保原文件不经 Nginx 静态暴露，并记录隔离接口行为；Verify：猜测 `/uploads/...` 地址取不到文件，本班下载经过 API 鉴权。

## 5. TXT/Markdown 上传与正文入库

- [x] 5.1 实现教师 POST /api/materials，以白名单校验 `.txt`/`.md`、流式上限和非空 UTF-8 正文；Verify：有效文件返回 201，空文件/非法编码/不支持扩展名返回 400，超限返回 413 且未读完整请求体。
- [x] 5.2 实现随机存储名、材料与知识库正文同一事务写入及失败补偿；Verify：成功时两表与文件均存在，注入写库或写盘故障后两表与目录均无残留。
- [x] 5.3 实现超期孤立临时文件清理；Verify：孤立文件最终清除，已提交材料文件不被误删。
- [x] 5.4 实现本班材料详情、正文与 GET /api/materials/{id}/file 下载，记录 API 行为；Verify：教师上传后同班学生可查看与下载，跨班请求返回 404，清空测试材料后列表为空。

## 6. 前端页面

- [x] 6.1 实现 React 登录页、材料列表、详情与教师上传入口，通过同源 `/api` 调用 Go；Verify：教师登录上传后本班学生可见，学生不出现上传入口，匿名访问引导登录。
- [x] 6.2 实现本班列表筛选、可读错误反馈与安全 Markdown 展示；Verify：筛选不出现跨班条目，上传失败不新增记录，恶意 Markdown 不执行脚本。
- [x] 6.3 配置 Vite 开发代理与 Nginx 生产反代使用相同 `/api` 路径；Verify：开发环境和 Compose 环境各调用一次 `/api/me`，前端不直连数据库或宿主 API 端口。

## 7. Compose 与运行保障

- [x] 7.1 编写 web（Nginx + 前端）、api（Go）、db（MySQL 8.0）的 Dockerfile、Nginx 与 Compose 配置；Verify：`docker compose config` 和干净构建通过，仅 web 映射宿主端口，上传卷只挂载 api。
- [x] 7.2 实现 API 等待数据库就绪与公开 GET /health 存活检查；Verify：正常进程返回 200/status=ok，停 db 后 /health 仍为 200 而材料 API 返回 503，恢复 db 后业务可用。
- [x] 7.3 在根目录 README 写明范围与不做项、环境变量、种子账号、Compose 启动、单实例、跨班 404、备份及不删卷回滚；Verify：在干净目录按 README 从零启动，仓库、镜像、前端与日志不含实际密钥。
- [x] 7.4 执行不带 `-v` 的 Compose 停止/重启和容器重建；Verify：账号、原文件、知识库正文仍可登录、查看与下载。

## 8. 跨模块验收与规格校验

- [x] 8.1 执行教师上传→本班列表与知识库正文可查→同班学生查看/下载；Verify：记录命令与输出，材料 ID、班级和正文一致。
- [x] 8.2 执行匿名访问、学生上传和 A/B 跨班 ID 三条负向路径；Verify：分别得到 401、403、404，响应不泄露材料内容，失败后数据库与上传目录不变。
- [x] 8.3 对照四份 spec 的 Scenario 记录通过/不通过及证据，并执行 `openspec validate add-class-auth-knowledge-base --strict`；Verify：严格校验退出码为 0，全部必做场景有可复核结果后才勾选完成。
