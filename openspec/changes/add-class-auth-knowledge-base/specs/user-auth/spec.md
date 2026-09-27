# Spec Delta

## Purpose

为教师和学生提供统一的账号密码登录及受保护资源认证机制，明确页面跳转、接口错误、会话生命周期和密码存储约束，使客户端无法绕过服务端身份验证。

## ADDED Requirements

### Requirement: Account password login
系统 SHALL 允许预置的教师和学生用账号密码登录，并从服务端账户记录确定角色和唯一班级。失败响应 MUST 不透露账号是否存在。

#### Scenario: Both roles can log in
- **WHEN** 教师或学生提交正确账号密码
- **THEN** 返回成功并创建会话，当前用户接口返回对应角色及班级，不返回密码哈希

#### Scenario: Invalid credentials
- **WHEN** 提交不存在的账号或错误密码
- **THEN** 均返回 401 和一致的凭据错误，不创建会话

#### Scenario: Rate-limited credentials
- **WHEN** 同一用户名与来源地址多次登录失败并触发限流
- **THEN** 锁定期内登录仍返回与错误密码一致的状态码和响应体，不透露账号是否存在

### Requirement: Pre-provisioned class accounts
系统 SHALL 提供仅服务端可运行的幂等种子流程，预置 A/B 两班及教师 A、学生 A1、学生 B1；各账号口令 SHALL 从服务端环境变量读取并在保存前哈希，MUST 不提供共享默认密码。重复启动 MUST 不复制账号、班级或材料，也不得覆盖教师已上传的内容；公开请求 MUST 不能创建账号或更改角色与班级。

#### Scenario: Provision teacher and student
- **WHEN** 运维人员为 A 班教师和学生分别提供环境变量口令并执行种子流程
- **THEN** 两个账号可用对应密码登录，当前用户接口返回各自角色与 A 班标识，持久化数据仅含密码哈希

#### Scenario: Unprovisioned account
- **WHEN** 未预置用户尝试登录或通过公开 HTTP 请求创建账号
- **THEN** 登录返回 401，公开请求不能创建账号、班级或身份关系

#### Scenario: Two-class acceptance accounts
- **WHEN** 验收环境通过服务端种子流程创建班级 A、班级 B、教师 A、学生 A1 和学生 B1，并从环境变量为各账号提供口令
- **THEN** 教师 A 与学生 A1 登录后归属 A 班，学生 B1 登录后归属 B 班；三者均可用自己的密码登录，不能用其他账号的密码登录

#### Scenario: Idempotent seed
- **WHEN** 使用同一配置重复执行种子流程或重启服务
- **THEN** 账号、班级和验收材料不重复，教师已上传内容不被覆盖

### Requirement: Protected resources require authentication
系统 SHALL 对所有材料页面和材料 API 验证会话；未登录页面请求跳转登录页，API 返回 JSON 401。

#### Scenario: Anonymous page navigation
- **WHEN** 未登录用户访问材料列表页面
- **THEN** 跳转登录页且不返回材料内容；登录后可进入本班材料页

#### Scenario: Invalid API session
- **WHEN** 无会话、伪造会话或过期会话请求材料 API
- **THEN** 返回 401，不执行上传或读取操作

### Requirement: Secure password and session handling
系统 MUST 使用带独立盐的自适应密码哈希，禁止明文或可逆密码存储；会话 SHALL 有有效期、使用 HttpOnly Cookie，并在生产 HTTPS 环境启用 Secure。登出 SHALL 使服务端会话失效，Cookie 认证的写操作 SHALL 防止 CSRF。

#### Scenario: Password storage verification
- **WHEN** 创建两个使用相同密码的测试账号并检查持久化数据与日志
- **THEN** 两个哈希不同且都可验证原密码，数据库和日志中无明文密码

#### Scenario: Logout invalidates session
- **WHEN** 用户登出后重放原会话访问受保护 API
- **THEN** 返回 401

#### Scenario: Forged cross-site upload
- **WHEN** 使用有效会话但缺少有效 CSRF 凭据发起上传
- **THEN** 拒绝请求且无材料或知识库正文写入
