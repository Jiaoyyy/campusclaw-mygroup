# Proposal

## Why

第 3 课已能按班级上传、查看和下载教学材料，但列表只能按文件名筛选。师生需要从本班材料正文中找到相关内容，并能核对命中内容来自哪份原始材料和哪个位置。

## What Changes

- 为已登录师生提供本班材料正文的关键词检索，不允许请求指定或覆盖班级。
- 命中结果返回材料 ID、标题、摘录、段号及原文字符范围，并可打开已有的受保护材料详情。
- 无命中时明确显示空结果；匿名请求、无效查询和伪造班级请求分别按认证、校验和授权规则拒绝。
- 本次 MVP 不新增向量数据库、嵌入、混合检索或生成式回答；这些可在后续变更中扩展。

## Capabilities

### New Capabilities

- `knowledge-retrieval`: 本班正文关键词检索、可追溯命中结果及失败路径。

### Modified Capabilities

无；沿用第 3 课已有的认证、班级和材料读取规则。

## Impact

新增同源 `GET /api/retrieval?q=...`、Go 检索服务与材料页检索区。读取现有 MySQL `knowledge_entries` 和 `materials`，不改上传流程、表结构或 Compose 服务。
