# Knowledge Retrieval

## ADDED Requirements

### Requirement: Indexed, traceable class-scoped retrieval
系统 SHALL 为已登录师生提供本班 `keyword`、`vector` 和 `hybrid` 三种检索模式，默认 `hybrid`。班级 MUST 仅取自服务端会话；客户端班级或角色声明 MUST NOT 改变检索范围。无候选时 SHALL 返回空 `hits` 与「资料中未找到相关内容」。

#### Scenario: Keyword search
- **WHEN** 用户在 keyword 模式检索本班已保存切片（包括向量生成失败的切片）
- **THEN** 仅使用 MySQL ngram 全文索引，且不调用嵌入网关或 Qdrant

#### Scenario: Embedding gateway not configured yet
- **WHEN** 服务端尚未配置嵌入模型
- **THEN** 材料仍切分并保存于 MySQL，关键词检索可用且为默认模式；向量、混合检索与问答明确返回 503，不伪装成语义检索

#### Scenario: Vector and hybrid search
- **WHEN** 用户在 vector 或 hybrid 模式检索
- **THEN** 向量按会话班级过滤，余弦低于 0.35 的切片被排除；hybrid 在两路过滤后以 k=60 的 RRF 融合

#### Scenario: Cross-class isolation
- **WHEN** A 班用户在请求中声明 B 班编号或使用仅存在于 B 班材料中的词
- **THEN** 仍只检索 A 班，响应不含 B 班的标题、正文、编号或摘录

### Requirement: Durable chunk indexing
系统 SHALL 保留原文件和知识库原文，并把切片正文、序号、相对于待切分文本的字符范围和状态存于 MySQL；向量点 ID SHALL 与切片 ID 相同，payload SHALL 不含正文。默认 `auto` 为最大 800 字、重叠 80 字；`custom` 与 `hierarchy` SHALL 可选。教师 SHALL 能对本人班级材料按新策略重建索引。

#### Scenario: Embedding failure
- **WHEN** 嵌入或 Qdrant 写入失败
- **THEN** 原材料保留，对应切片标为 failed，不提供伪造的向量结果

### Requirement: Verifiable source and grounded answer
每条命中 SHALL 含材料标题、可打开的材料 ID、切片序号、字符范围和取自 MySQL 切片正文的摘录。问答 SHALL 先以本班混合检索取前四条；没有命中 MUST 返回「资料中未找到相关内容」和空引用且 MUST NOT 调用对话模型。

#### Scenario: Answer with evidence
- **WHEN** 本班检索有候选切片
- **THEN** 对话模型只收到问题及本班切片标题、序号和正文，返回的出处仅包含回答中实际引用的切片，且 `[1]` 等编号与出处编号一致
