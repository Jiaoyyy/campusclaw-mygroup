# Spec Delta

## Purpose

将教师上传的 TXT 和 Markdown 教学材料连同提取出的正文持久化为本班知识库记录，让本班教师与学生能从真实数据库列表中找到材料、查看正文并经授权下载原文件。

## ADDED Requirements

### Requirement: Durable material upload and listing
系统 SHALL 接收 UTF-8 TXT 和 Markdown 文件，单文件上限由服务端环境变量配置。成功接收 SHALL 返回材料 ID，且原文件、材料记录和对应的本班知识库正文均已持久化。本班列表 SHALL 从数据库读取并包含该材料 ID、文件名、上传者和时间，不得使用硬编码演示条目。

#### Scenario: Accepted upload appears in class list
- **WHEN** 教师上传包含有效 UTF-8 正文的 TXT 或 Markdown 文件并收到成功响应
- **THEN** 本班教师与学生刷新列表可看到同一材料记录，可经授权查看正文及下载原文件

#### Scenario: Persisted knowledge base record
- **WHEN** 教师成功上传有效材料后检查本班材料列表与知识库记录
- **THEN** 列表中出现该材料 ID，知识库中存在与该材料及班级关联的提取正文

#### Scenario: Empty material list
- **WHEN** 本班尚未上传任何材料或其材料记录已在测试环境清空
- **THEN** 本班列表为空，不出现硬编码演示材料

### Requirement: Validate upload and keep storage consistent
系统 SHALL 以 .txt/.md 扩展名白名单校验文件、在读取完整超限请求体前执行服务端配置的大小限制，并校验非空 UTF-8 正文。空文件、无可提取正文、无效编码或正文入库失败 MUST 被拒绝；拒绝或失败后 MUST 不留下可见材料记录、知识库正文或原文件。错误响应 SHALL 不泄露内部路径与堆栈。

#### Scenario: Rejected file
- **WHEN** 教师上传空文件、不支持的扩展名或超过服务端配置上限的文件
- **THEN** 分别返回 400、400 或 413，且材料列表、知识库与文件存储均无新增残留

#### Scenario: Invalid text or persistence failure
- **WHEN** 教师上传无有效 UTF-8 正文的文件，或正文入库过程中发生错误
- **THEN** 上传失败并返回可读错误；材料记录、知识库正文和原文件均不留残余

### Requirement: Authorized material reading
系统 SHALL 允许本班教师和学生查看本班材料详情、提取正文及下载原文件。Markdown 正文在页面展示时 SHALL 安全渲染，不得执行材料中的脚本。

#### Scenario: Read own class material
- **WHEN** 本班教师或学生请求已成功上传的本班材料
- **THEN** 可查看材料详情和正文，并经授权下载原文件

#### Scenario: Safe Markdown display
- **WHEN** 用户查看含有 HTML 或脚本样式文本的 Markdown 材料
- **THEN** 页面不执行材料中的脚本
