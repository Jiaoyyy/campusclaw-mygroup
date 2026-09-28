# Design

## Context

第 3 课把原文件和 UTF-8 正文存于 MySQL，第 4 课 MVP 在 Go 中扫描完整正文。完整第 4 课在现有会话和班级授权后接入持久切片、全文索引、嵌入网关、Qdrant 和对话网关。浏览器仍只访问同源 Go API。

## Decisions

1. `knowledge_chunks` 保存切片正文、序号、相对于待切分文本的 Unicode 字符范围及索引状态，采用 MySQL ngram FULLTEXT（2 字 token）。原文和原文件保持不变。已有材料由启动时幂等补建默认切片。
2. `auto` 默认最大 800 字、重叠 80 字并优先在空行/换行/句号断开；`custom` 可选分隔符、100–2000 字长度、0–50% 重叠和 URL/邮箱/空白预处理；`hierarchy` 按 Markdown 三级标题分章，超长章节再切分。教师可重建本人班级材料索引。
3. 服务端调用兼容 OpenAI 的嵌入接口，得到向量后写入 Qdrant `campusclaw_chunks`，点 ID 等于切片 ID；payload 仅存班级、材料、知识条目、切片 ID 与序号。嵌入失败保留原文，切片置为 `failed`；Qdrant 不可用时关键字查询独立运行。
4. `GET /api/retrieval` 增加 `mode=keyword|vector|hybrid`，默认 `hybrid`。关键字仅查 MySQL 全文索引；向量查询先嵌入再在 Qdrant 按会话班级过滤，低于 0.35 的余弦候选丢弃，回表时再次核验班级；混合模式在两路绝对过滤后以 RRF（k=60）融合。客户端提供的班级或角色字段不参与授权。
5. `POST /api/ask` 用最新问题做本班混合检索，最多取前四条；无候选直接返回固定文案和空引用，不调用对话模型；有候选时只传标题、切片序号与正文，返回与命中顺序一致的引用列表。网关密钥只在服务端环境变量中配置。

## Risks and recovery

索引由 MySQL 与 Qdrant 两库组成，无法跨库原子提交。先保存原文和切片，再嵌入/写向量并标记 `ready`；失败保留切片为 `failed`，教师重建可清理旧向量后重试。缺少模型配置时向量、混合及问答返回 503，不伪装为成功。密钥不进入日志或响应。
