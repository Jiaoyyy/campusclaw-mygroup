# Spec Delta

## Purpose

让已登录的教师和学生从本人班级的教学材料正文中检索原文词语，并对每条命中提供可核对的材料来源、位置和摘录，同时防止查询泄露其他班级的内容。

## ADDED Requirements

### Requirement: Authenticated class-scoped keyword retrieval
系统 SHALL 允许已登录教师和学生按关键词检索本班已成功入库的材料正文。检索范围 MUST 由服务端会话中的班级决定，客户端传入的班级或角色声明 MUST 不能扩大范围。

#### Scenario: Same-class match
- **WHEN** A 班学生检索 A 班已入库材料正文中的词语
- **THEN** 返回包含该词语的 A 班命中结果

#### Scenario: Cross-class isolation
- **WHEN** B 班学生检索只存在于 A 班材料中的词语
- **THEN** 返回空命中，响应不包含 A 班材料的标题、正文、材料 ID 或摘录

#### Scenario: Forged class
- **WHEN** 用户在检索请求中声明其他班级或伪造角色
- **THEN** 请求被拒绝，且不返回其他班级内容

### Requirement: Traceable results
每条命中 SHALL 包含可打开的材料 ID、原始文件名、命中所在段号、以 Unicode 字符计数的原文起止位置和取自原文的摘录。材料详情仍 SHALL 经已有班级授权检查。

#### Scenario: Inspect source
- **WHEN** 用户打开一条本班检索命中
- **THEN** 能查看对应材料正文，并根据结果中的位置与摘录核对来源

### Requirement: Clear empty and invalid outcomes
无命中时系统 SHALL 返回空命中集合并在页面明确提示；匿名访问 MUST 返回 401，空白或过长的查询 MUST 返回 400，错误响应 MUST 不泄露其他班级正文。

#### Scenario: No matching text
- **WHEN** 用户检索本班材料中不存在的词语
- **THEN** 命中集合为空，页面显示未找到相关内容

#### Scenario: Unauthenticated or invalid query
- **WHEN** 匿名用户发起检索，或已登录用户提交空白或超过 100 个 Unicode 字符的查询
- **THEN** 匿名请求返回 401，无效查询返回 400
