# Spec Delta

## Purpose

以班级作为教学材料、原始文件和知识库正文的数据边界，并将教师和学生的操作权限落实在服务端，保证隐藏按钮或客户端提供的身份信息不影响授权判断。

## ADDED Requirements

### Requirement: Single class server authority
每个教师和学生账号 MUST 关联唯一班级。系统 SHALL 从已认证账号读取角色和班级，拒绝客户端覆盖身份范围的请求；无有效班级归属的账号 MUST 被拒绝访问受保护业务资源。

#### Scenario: Forged class or role
- **WHEN** A 班用户在查询参数、请求体或自定义头中声明 B 班或伪造教师角色
- **THEN** 请求被拒绝，不能读取或写入 B 班数据，也不能提升角色

### Requirement: Teacher-only upload
上传 API SHALL 仅允许已认证教师上传本班材料；学生调用上传 API MUST 返回 403，且授权拒绝不得产生持久化文件、材料或知识库正文。

#### Scenario: Student calls endpoint directly
- **WHEN** 已登录学生绕过页面以有效 CSRF 凭据直接调用上传 API
- **THEN** 返回 403，无任何知识库写入

#### Scenario: Teacher uploads to own class
- **WHEN** 已登录教师提交合规材料
- **THEN** 新材料绑定该教师的班级及上传者身份

#### Scenario: Student cannot gain teacher role
- **WHEN** 已登录学生直接调用教师上传 API，且在请求体或请求头声称自己是教师
- **THEN** 服务端仍按预置角色返回 403，不产生文件、材料或知识库正文

### Requirement: Class isolation for every read path
列表、详情、知识库正文和下载 SHALL 在服务端限制到当前用户班级，包含返回条目、总数及正文。跨班材料 ID 的直接访问 MUST 返回与不存在 ID 一致的 404，禁止暴露公开文件地址。

#### Scenario: Cross-class resource access
- **WHEN** A 班教师或学生使用已知 B 班材料 ID 请求详情、正文或下载
- **THEN** 返回 404 且无 B 班内容或元数据

#### Scenario: Cross-class upload target
- **WHEN** A 班教师上传材料时提交 B 班的班级标识
- **THEN** 请求被拒绝，不在 A 班或 B 班创建材料

#### Scenario: Filtered lists and knowledge entries
- **WHEN** 两个班级都有材料且 A 班用户请求列表或 B 班材料正文
- **THEN** 列表与计数不包含 B 班材料，正文请求返回 404 且不包含 B 班文本

#### Scenario: Two-class acceptance materials
- **WHEN** 验收数据包含班级 A、班级 B、教师 A、学生 A1、学生 B1，以及分别归属 A 班和 B 班的两条材料记录
- **THEN** 教师 A 和学生 A1 的列表只包含 A 班材料，学生 B1 的列表只包含 B 班材料；学生 A1 请求 B 班材料 ID 返回 404，学生 B1 请求 A 班材料 ID 返回 404

#### Scenario: Same class read access
- **WHEN** 本班教师或学生访问本班材料列表、详情、正文或下载
- **THEN** 获得权限允许的本班结果
