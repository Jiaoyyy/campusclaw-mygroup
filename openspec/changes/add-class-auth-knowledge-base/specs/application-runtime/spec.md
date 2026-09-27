# Spec Delta

## Purpose

为教学知识库提供可重复的 Docker Compose 启动流程、服务端配置与持久化保障，并通过无需登录的健康检查接口让部署人员识别核心依赖故障，同时避免暴露密钥及内部连接信息。

## ADDED Requirements

### Requirement: Compose startup and persistence
项目 SHALL 在仓库根目录 README 提供 Docker Compose 配置、种子初始化和启动说明，配置完成后使用 docker compose up --build 启动 web（Nginx 与前端构建产物）、api（Go）和 db（MySQL）。只有 web SHALL 映射宿主端口，上传文件卷只供 api 使用。普通停止、重新启动以及不删除具名卷的容器重建 MUST 保留账号、材料文件和知识库正文。

#### Scenario: Fresh deployment
- **WHEN** 运维人员按文档配置服务端环境变量并执行启动及账号初始化步骤
- **THEN** 教师和学生可登录，教师上传后材料与提取正文可由本班用户读取

#### Scenario: Required seed credentials
- **WHEN** 全新部署缺少必需的服务端种子口令或会话密钥
- **THEN** 启动失败且不采用内置共享口令或密钥

#### Scenario: Restart persistence
- **WHEN** 已上传材料并保存正文后执行不删除卷的 Compose 停止和重启
- **THEN** 原账号仍可登录，原材料可列出、查看正文及下载

#### Scenario: Rebuild containers without deleting volumes
- **WHEN** 已预置两班账号和材料后，按仓库根目录 README 执行 Compose 构建并重建容器，且不删除具名卷
- **THEN** 应用可访问，GET /health 在依赖就绪后返回 200 和 status=ok；原账号仍可登录，两班各自材料仍可列出、查看正文及下载

### Requirement: Server-only secrets
密钥 MUST 只通过服务端运行时环境变量提供，禁止硬编码、提交实际密钥、写入镜像构建参数或前端资源。缺少必需密钥时服务 MUST 拒绝以默认密钥启动；示例配置 SHALL 仅含占位符。

#### Scenario: Secret not exposed
- **WHEN** 检查源码、构建产物、浏览器响应、日志和健康检查响应
- **THEN** 不包含实际密钥、密码或带凭据连接串

#### Scenario: Missing required secret
- **WHEN** 未配置必需服务端密钥启动服务
- **THEN** 启动失败并指出缺少的配置名称，不输出密钥值

### Requirement: Public health endpoint
应用 SHALL 提供不需登录的 GET /health 作为进程存活检查，能响应时返回 HTTP 200 与 JSON status=ok，不将数据库探活并入该端点。数据库或文件存储不可用时，受影响的业务 API SHALL 返回 503，不得将已登录用户误判为 401。响应 MUST 不泄露凭据、堆栈或内部连接地址。

#### Scenario: Healthy application
- **WHEN** 未登录客户端在依赖就绪后请求 GET /health
- **THEN** 收到 200 和 status=ok，无跳转

#### Scenario: Database unavailable
- **WHEN** 数据库不可连接但应用进程仍能响应
- **THEN** GET /health 仍返回 200 和 status=ok；材料业务 API 返回 503，而非 401

#### Scenario: Upload storage unavailable
- **WHEN** 上传文件存储不可用但应用进程仍能响应
- **THEN** GET /health 仍返回 200 和 status=ok；受影响的上传或下载 API 返回 503
