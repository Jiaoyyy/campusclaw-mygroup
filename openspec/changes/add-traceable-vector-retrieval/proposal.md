# Proposal

## Why

当前第 4 课 MVP 仅扫描完整正文做字面匹配，缺少持久切片、全文索引、向量与混合检索，以及有证据的简短回答。

## What Changes

- 上传与种子补齐时将原文按策略切分，持久化切片与状态，并支持教师重建索引。
- 用 MySQL ngram FULLTEXT、Qdrant 余弦向量、RRF 混合排序实现三种本班检索模式。
- 命中结果回到 MySQL 提供出处；问答仅在本班命中切片后调用对话模型。
- 嵌入与对话网关仅由 Go 服务端调用，地址、模型和密钥从环境变量配置。

## Capabilities

### Modified Capabilities

- `knowledge-retrieval`: 完整第 4 课可追溯检索与简短问答。

## Impact

新增切片表与全文索引、Qdrant 服务、模型网关配置、重建与问答 API，以及页面上的策略、模式和出处操作。沿用第 3 课认证和材料原文存储。
