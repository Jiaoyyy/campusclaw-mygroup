# Token Authentication

## ADDED Requirements

### Requirement: Browser Bearer token login
系统 SHALL 允许预置账号通过 `POST /api/login` 获取随机访问令牌；成功响应 SHALL 包含令牌类型、令牌和到期时间，不设置登录 Cookie。浏览器后续受保护请求 SHALL 使用 `Authorization: Bearer`，且令牌不得出现在 URL 中。服务端 SHALL 不接受 Cookie 作为登录凭据。

#### Scenario: Successful login and protected request
- **WHEN** 用户以正确账号口令获取 token 并携带返回的 Bearer 令牌访问 `/api/me` 和本班材料
- **THEN** 服务端从会话与用户表确认身份、角色及班级，并仅返回本班资源

#### Scenario: Missing or invalid token
- **WHEN** 请求无令牌，或 Authorization 头包含伪造、格式错误、过期的 Bearer 令牌
- **THEN** 受保护 API 返回 401；即使请求同时携带 Cookie 也不得回退

### Requirement: Immediate revocation and usable download
系统 SHALL 在登出时删除对应会话，使已发出的 Bearer 令牌立即失效。浏览器下载原文件 SHALL 通过携带 Bearer 头的鉴权请求完成。

#### Scenario: Logout revokes token
- **WHEN** 已登录用户带有效 Bearer 令牌和现有 CSRF 凭据登出
- **THEN** 同一令牌再次访问 `/api/me` 返回 401

#### Scenario: Authorized material download
- **WHEN** 用户通过页面下载本班原文件
- **THEN** 下载请求携带 Bearer 头并通过已有班级检查；无令牌请求返回 401
