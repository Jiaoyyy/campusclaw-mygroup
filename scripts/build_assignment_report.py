"""Build the submission PDF from verified network evidence, never invented returns.

Run with the bundled Python runtime (reportlab, pypdf and Pillow required).
"""
from pathlib import Path
import json
import os
from xml.sax.saxutils import escape

from reportlab.lib import colors
from reportlab.lib.pagesizes import A4
from reportlab.lib.styles import ParagraphStyle
from reportlab.pdfbase import pdfmetrics
from reportlab.pdfbase.ttfonts import TTFont
from reportlab.platypus import SimpleDocTemplate, Paragraph, Spacer, Table, TableStyle, PageBreak, Image
from pypdf import PdfReader, PdfWriter

ROOT = Path(__file__).resolve().parents[1]
EVIDENCE = ROOT / 'output/assignment-evidence'
DATA = json.loads((EVIDENCE / 'verification.json').read_text())
CATALOG = json.loads((EVIDENCE / 'catalog.json').read_text())
REVISION = os.getenv('ASSIGNMENT_CODE_REVISION', '')
VERSION_TEXT = ('代码版本：' + REVISION) if REVISION else '验证版本：当前工作区构建，尚未标注远端代码提交'
OUT = ROOT / 'output/pdf/campusclaw-personal-ai-platform-verification.pdf'
OUT.parent.mkdir(parents=True, exist_ok=True)
TEMP = ROOT / 'tmp/pdfs/assignment-report-body.pdf'
TEMP.parent.mkdir(parents=True, exist_ok=True)
pdfmetrics.registerFont(TTFont('Body', '/System/Library/Fonts/Supplemental/Songti.ttc', subfontIndex=6))
pdfmetrics.registerFont(TTFont('BodyBold', '/System/Library/Fonts/Supplemental/Songti.ttc', subfontIndex=1))
pdfmetrics.registerFontFamily('Body', normal='Body', bold='BodyBold', italic='Body', boldItalic='BodyBold')
TEAL = colors.HexColor('#087C73')
INK = colors.HexColor('#20383F')
GRAY = colors.HexColor('#617579')
styles = {
    'title': ParagraphStyle('title', fontName='BodyBold', fontSize=25, leading=37, textColor=INK, spaceAfter=18, wordWrap='CJK'),
    'h1': ParagraphStyle('h1', fontName='BodyBold', fontSize=18, leading=27, textColor=TEAL, spaceAfter=14, keepWithNext=True, wordWrap='CJK'),
    'h2': ParagraphStyle('h2', fontName='BodyBold', fontSize=12, leading=19, textColor=INK, spaceBefore=10, spaceAfter=6, keepWithNext=True, wordWrap='CJK'),
    'body': ParagraphStyle('body', fontName='Body', fontSize=10, leading=17, textColor=INK, spaceAfter=9, wordWrap='CJK'),
    'small': ParagraphStyle('small', fontName='Body', fontSize=8.4, leading=13, textColor=GRAY, spaceAfter=7, wordWrap='CJK'),
    'code': ParagraphStyle('code', fontName='Body', fontSize=8.2, leading=12.5, textColor=INK, backColor=colors.HexColor('#F1F6F5'), borderPadding=9, spaceBefore=5, spaceAfter=12, wordWrap='CJK'),
    'cell': ParagraphStyle('cell', fontName='Body', fontSize=9, leading=14, textColor=INK, wordWrap='CJK'),
}
story = []


def p(text, style='body'):
    story.append(Paragraph(escape(str(text)).replace('\n', '<br/>'), styles[style]))


def code(value):
    text = value if isinstance(value, str) else json.dumps(value, ensure_ascii=False, indent=2)
    # Paragraphs can split across pages; preserve indentation and every character.
    p(text.replace('\t', '  '), 'code')


def table(rows, widths):
    data = [[Paragraph(escape(str(cell)).replace('\n', '<br/>'), styles['cell']) for cell in row] for row in rows]
    item = Table(data, colWidths=widths, repeatRows=1, hAlign='LEFT')
    item.setStyle(TableStyle([('BACKGROUND', (0,0), (-1,0), colors.HexColor('#DCEFEB')), ('ROWBACKGROUNDS', (0,1), (-1,-1), [colors.white, colors.HexColor('#F5F8F7')]), ('VALIGN', (0,0), (-1,-1), 'TOP'), ('LEFTPADDING', (0,0), (-1,-1), 9), ('RIGHTPADDING', (0,0), (-1,-1), 9), ('TOPPADDING', (0,0), (-1,-1), 8), ('BOTTOMPADDING', (0,0), (-1,-1), 8), ('LINEBELOW', (0,0), (-1,0), .5, TEAL)]))
    story.append(item)
    story.append(Spacer(1, 12))


def page(title):
    if story:
        story.append(PageBreak())
    p(title, 'h1')


def llm(case):
    return [entry for entry in case['network'] if entry['kind'] == 'llm']


def case(name):
    return next(item for item in DATA['cases'] if item['name'] == name)


normal = case('01_prompt_template')
history = case('02_conversation_history')
kb = case('03_knowledge_skill')
external = case('04_mcp_tool')
kb_meta = next(item['data'] for item in kb['events'] if item['event'] == 'meta')
selection, final = llm(external)
calls = [entry for entry in external['network'] if entry['kind'] == 'mcp' and (entry['request'] or {}).get('method') == 'tools/call']

page('个人 AI 应用平台验证报告')
p('CampusClaw\n教学助手与知识库平台', 'title')
p('作者：焦阳　　验证日期：2026-10-27')
p('Git 仓库：https://github.com/Jiaoyyy/campusclaw-mygroup')
p('平台面向教师与学生，按班级隔离材料、技能、应用和私有会话。教师配置应用角色、知识库与外部工具，学生选择应用提问，并以流式方式查看回答和材料出处。')
table([
    ['作业功能要求', '本次实现与验证'],
    ['内置多个 AI 应用', '解题引导助手、知识复习助手、学习计划助手，各自有提示词和技能绑定'],
    ['提示词模板', '3 个登录可访问的模板，可选择、编辑、保存到应用'],
    ['知识库检索', '关键词、向量、混合检索；按班级和应用材料绑定过滤'],
    ['内置及上传 Skill', '3 份内置 Markdown；支持导入、编辑、原文保存与导出'],
    ['外部工具及 MCP', 'Streamable HTTP 客户端、工具允许列表、应用绑定与实际 tools/call'],
], [135, 360])
p('验证结论：以下五项均取得实际请求与返回记录。所选模板会进入生成请求；知识库仅发送检索切片；选中技能的完整原文进入生成请求；Skill 通过提示词约束模型行为；MCP 使用“工具选择 - 实际调用 - 回答生成”的两次 LLM 请求流程。')
p(VERSION_TEXT + ('\n对应代码：https://github.com/Jiaoyyy/campusclaw-mygroup/tree/' + REVISION if REVISION else '\n远端同步状态以最终交付说明为准。'), 'small')

page('1. 验证方法与环境')
table([
    ['项目', '实测环境'],
    ['前端 / API / 数据', 'React + TypeScript + Nginx / Go net/http / MySQL 8 + Qdrant'],
    ['对话网关', DATA['gateway'] + '/chat/completions'],
    ['模型名', DATA['model']],
    ['独立验证入口', 'http://127.0.0.1:18382；Compose 项目 campusclaw-assignment-check'],
    ['本次知识库检索模式', '关键词检索；独立环境关闭嵌入，避免长资料同步索引超时'],
    ['测试资料', f'人工编写“紫杉协议”资料，共 {DATA["source_material_characters"]} 个 Unicode 字符，12 节'],
    ['外部 MCP', '仓库公开术语演示服务，实际使用 MCP SDK 和 Streamable HTTP；工具 course_glossary'],
], [125, 370])
p('记录器位于 API 与真实课程对话网关之间，保存请求 JSON 和返回 JSON/SSE 正文。模型返回来自课程网关；MCP 结果来自本地公开术语服务。记录器不保存 Authorization、Cookie、API Key 或登录口令。')
p('网络说明：本机虚拟 DNS 解析到不可达地址，本次记录进程按公开 DNS 结果临时指定网关服务器 IP，仍使用原域名并校验 HTTPS 证书；未修改系统 DNS。', 'small')
p('通过实际 /api/login、/api/materials、/api/skills、/api/agents、/api/conversations 和消息接口完成操作，未用手写 JSON 代替模型返回。测试使用独立数据卷，未读取原项目的上传资料。')
table([
    ['用例', '目的'],
    ['01_prompt_template', '无知识库与技能时，核对模板、请求顶层参数和真实回答'],
    ['02_conversation_history', '同一会话第二轮，核对前一轮 user / assistant 是否进入 messages'],
    ['03_knowledge_skill', '绑定长资料及完整 Skill，核对切片数量与技能首尾内容'],
    ['04_mcp_tool', '核对工具定义、LLM tool_calls、MCP tools/call 和最终生成请求'],
], [155, 340])
p('完整网络记录作为 PDF 附件 verification-evidence.json 嵌入，同时提供独立 JSON。正文中的“节选”只用于排版；未经节选的请求、响应与 SSE 事件以附件为准。')

page('2. 每轮对话发送什么？模板是否传递？')
p('结论：最终回答请求包含 model、messages、stream:true、temperature:0。所保存的完整角色提示词放在 messages[0].content，之后追加本轮技能和引用边界。模板选择会把模板文本复制进应用的 prompt，并在每轮重新组装时传递。', 'body')
p('普通对话：实际完整请求体', 'h2')
code(llm(normal)[0]['request'])
p('真实返回与前端收到的内容', 'h2')
p(normal['answer'])
p(f'网关 HTTP 状态：{llm(normal)[0]["status"]}；Content-Type：{llm(normal)[0]["response_content_type"]}。网关以 choices[].delta.content 输出片段；后端将其转为 meta、delta、done 等 SSE 事件，最后写入消息表。', 'small')
p('请求未显式设置 max_tokens、top_p、seed 或 response_format；这些字段不能在报告中写成已发送参数。认证信息位于 HTTP Authorization 头，正文与报告不展示凭据。', 'small')

page('3. 多轮历史与返回格式')
messages = llm(history)[0]['request']['messages']
table([['messages 顺序', '角色', '实际内容']] + [[str(i), item['role'], item['content']] for i, item in enumerate(messages)], [72, 70, 353])
p('第二轮真实回答', 'h2')
p(history['answer'])
p('后端从数据库读取最近最多六条非空 user / assistant 消息，每条最多保留 500 个字符。由于当前问题也已入库，组装器会先去掉历史末尾的当前问题，再追加带证据的当前 user 消息，避免重复。技能与证据不会作为独立历史消息保存后原样重放，而会按本轮绑定重新组装。')
p('网关 SSE 返回格式：真实事件摘录', 'h2')
response = llm(normal)[0]['response']
if isinstance(response, str):
    lines = [line for line in response.splitlines() if line.startswith('data:')]
    code('\n'.join(lines[:2] + lines[-2:]))
else:
    code(response)
p('浏览器接收的是本应用包装后的 SSE，含技能名、引用和工具名等元数据；这与网关原始响应格式不同。完整原始 SSE 及应用 SSE 事件都已记录在附件中。', 'small')

page('4. 绑定知识库后，会发送全部内容吗？')
p('结论：不会。本应用先按用户班级和绑定材料检索，最终回答只加入本轮选出的最多四个切片，每个切片最多 800 个 Unicode 字符；既不直接拼接整个知识库，也不把数据库或向量库整体发给对话模型。')
table([
    ['核对项', '实测结果'],
    ['原资料总长度', f'{DATA["source_material_characters"]} 字符'],
    ['进入本轮的切片数', str(DATA['selected_chunk_count'])],
    ['进入请求的切片正文长度', f'{DATA["selected_chunk_characters"]} 字符（不含标题和编号）'],
    ['资料末尾唯一标记', 'END_OF_KNOWLEDGE_FILE_7654321 未出现在 LLM 请求中'],
    ['整份原文比较', '整份资料不是该请求的子串；附件可核对实际传递片段'],
    ['所发送的位置', 'messages 最后一条 user.content，带 [n]、材料名和切片序号'],
], [145, 350])
table([['切片序号', '字符范围', '来源']] + [[str(hit['chunk_index']), f'{hit.get("start_offset")} - {hit.get("end_offset")}', hit['title']] for hit in kb_meta['citations']], [90, 110, 295])
p('生成请求中的证据位置：实际节选', 'h2')
user_text = llm(kb)[-1]['request']['messages'][-1]['content']
code(user_text[:240] + '\n（此处节选；完整字段见 PDF 附件）')
p('补充区别：上传建索引时，切片正文会分批交给嵌入接口；每轮向量检索时，嵌入接口收到当前查询文本。嵌入调用与生成回答的 chat/completions 调用是两种不同请求，不能把“建立全库向量”理解成“每轮把全库发给对话模型”。')

page('5. 绑定 Skill 后，会发送完整技能吗？')
p('结论：选中技能时，当前实现发送该技能的完整 source_text；没有选中、停用或未绑定的技能不会因此自动进入生成请求。本次只绑定一份 Markdown 技能，实际请求包含其全部原文及末尾标记。')
p('上传并原样读回的完整技能文件', 'h2')
code(DATA['skill_text'])
table([
    ['验证项', '结果'],
    ['长度', f'{DATA["skill_characters"]} 字符'],
    ['原文读写比较', 'GET 返回的 source_text 与上传字符串相等'],
    ['请求中完整比较', '完整 source_text 是 system.content 的连续子串'],
    ['尾部标记', 'SKILL_END_1234567 存在'],
    ['SHA-256', DATA['skill_sha256']],
], [120, 375])
p('技能在最终回答请求中以“当前技能文件（教师维护的完整内容）”追加到 system 消息。当前实现没有按需只加载某些章节，也没有把绑定的全部技能一次性拼接。')
p('边界：多个技能时仍使用确定性的词法路由。Markdown 默认以标题作为可匹配名称，旧版技能使用适用条件词。多个技能的语义自动选择尚未实现；本报告不把它描述为模型自主技能规划。', 'small')

page('6. Skill 的执行逻辑是什么？')
table([
    ['阶段', '实际执行行为'],
    ['读取权限', '认证当前用户，核对本人、本班会话，再读当前应用绑定'],
    ['选择技能', '读取绑定且启用的技能；单个 Markdown 默认使用；其他情况按词法得分选至多一个，同分取较小 ID'],
    ['检索知识', '在本班绑定资料中检索，最多四条；旧技能可再按材料名过滤'],
    ['外部工具', '若应用绑定获准 MCP 工具，先进行工具选择和实际只读调用'],
    ['组装上下文', '角色提示词 + 完整选中技能 + 引用边界；再加入历史、当前问题与证据'],
    ['生成与保存', '发送 LLM，转发增量，按引用整理结果，并保存最终消息、技能名和实际工具名'],
], [100, 395])
p('因此，执行技能指的是服务端选择规则文件，再由 LLM 按其中的步骤和限制组织回答。上传 Markdown 不会在服务器上作为 Python、Shell 或任意程序执行；文件内写了某个工具名，也不会自动获得对应工具权限。知识检索由服务端执行，MCP 能力由允许列表和应用绑定共同决定。')
p('本次知识库 + 技能用例的真实回答', 'h2')
p(kb['answer'])
p('可见效果：回答围绕认证、授权和下一步检查给出提示。是否始终严格遵守教学规则仍取决于模型生成，不能仅凭一次响应声称所有问题都不会给出答案；报告验证的是上下文传递与本次执行行为。', 'small')
p('实测配置边界：若把限定某一主题的强约束技能绑定到通用工具应用，即使 MCP 调用成功，最终回答仍可能被技能牵引。当前单一 Markdown 技能默认生效，因此术语工具用例使用独立的复习应用，未绑定这份限定主题技能。', 'small')

page('7. 集成 MCP 后：第一次 LLM 请求')
p('结论：本项目采用两阶段调用。第一阶段只让模型选择获准工具，发送 stream:false、tool_choice:auto 以及 tools 定义；messages 中仅包含工具选择指令与当前问题。本阶段没有加入教学材料、完整 Skill、会话历史或应用角色模板。')
p('真实工具选择请求体（完整）', 'h2')
code(selection['request'])
p('真实工具选择返回体（完整）', 'h2')
code(selection['response'])

page('8. MCP 实际调用与第二次 LLM 请求')
p('后端解析 tool_calls，确认工具确实属于已绑定且在服务端允许列表中的工具，然后通过 MCP 执行 tools/call。以下是实际协议记录，不是发给 LLM 的 tools 定义。')
p('MCP 请求与返回（完整）', 'h2')
code(json.dumps(calls[0]['request'], ensure_ascii=False, separators=(',', ':')))
code(json.dumps(calls[0]['response'], ensure_ascii=False, separators=(',', ':')))
p('第二次 LLM 请求：顶层字段', 'h2')
table([['字段', '实际值 / 内容']] + [[key, str(value) if key != 'messages' else f'{len(value)} 条消息：角色、问题与工具结果；本用例未绑定 Skill 或知识库'] for key, value in final['request'].items()], [120, 375])
p('第二次请求恢复普通生成形式：stream:true、temperature:0，不再携带 tools 或 tool_choice；工具结果以 user.content 中的“外部 MCP 工具 course_glossary 的返回值”文本加入。当前实现没有使用 role:tool 或把前一轮 tool_calls 消息原样重放。')
final_text = final['request']['messages'][-1]['content']
marker = final_text.find('外部 MCP 工具')
code(final_text[marker:] if marker >= 0 else final_text)
p('真实最终回答', 'h2')
p(external['answer'])
p('MCP 服务仅收到工具名与 arguments，不收到应用登录令牌和整份班级资料。当前支持一个 MCP 服务，每轮最多调用一个文本工具；外部结果最多保留 4000 字符。', 'small')

page('9. 验证汇总、复现与交付')
table([
    ['需验证内容', '结论', '证据'],
    ['参数与返回；模板是否发送', '模板进入最终生成 system；实际返回已记录', '01、02 的 request / response / events'],
    ['知识库是否全部传递', '否；至多 4 个切片，每段至多 800 字符', '03 的 user.content 与资料尾标记比较'],
    ['技能是否全部传递', '选中时是；未选中时不会自动拼接', '03 完整原文比较及尾标记'],
    ['技能执行逻辑', '权限与路由、检索、上下文组装、模型生成', '源码与 03 的流式回答、存储结果'],
    ['MCP 请求参数', '先 tools/tool_choice，再 tools/call，最后 stream:true 生成', '04 的两次 LLM 及 MCP 请求/返回'],
], [140, 200, 155])
p('复现步骤', 'h2')
code('1. 按 README 配置本机 .env（不提交密钥）。\n2. docker compose build api web\n3. docker compose -f compose.yaml -f compose.mcp-demo.yaml build mcp-demo\n4. python3 scripts/verify_assignment.py\n5. 用含 reportlab / pypdf 的 Python 运行 scripts/build_assignment_report.py')
p('源码索引', 'h2')
p('backend/internal/aicatalog/：内置应用、模板与三份完整技能。\nbackend/internal/db/seed_apps.go：按班级初始化及保留已有修改。\nbackend/internal/tutor/service.go：模板与技能接口、原文解析保存。\nbackend/internal/tutor/conversations.go：路由、绑定检索、工具调用与历史。\nbackend/internal/tutor/chat.go：实际生成请求及 SSE 解析。\nbackend/internal/tutor/mcp.go：工具发现、选择、调用及过滤。\nscripts/verify_assignment.py：实际请求记录与断言。', 'small')
p('完整证据包含普通对话、第二轮历史、知识库与 Skill、MCP 两阶段请求及对应响应。PDF 已附加 verification-evidence.json；也可查看同目录交付的独立证据 JSON。网络记录无凭据，验证只使用合成资料。')
p('Git 仓库：https://github.com/Jiaoyyy/campusclaw-mygroup\n' + VERSION_TEXT, 'small')

if (EVIDENCE / 'ui-applications.png').exists():
    page('附录：浏览器操作实测')
    p('模板选择并保存', 'h2')
    p('浏览器选择“知识复习”模板、填入角色提示词并保存应用成功。下图同时展示多个内置应用与模板入口。', 'small')
    story.append(Image(str(EVIDENCE / 'ui-applications.png'), width=495, height=270))
    p('完整 Markdown 导入', 'h2')
    p('通过文件选择器导入后，编辑器内容与原文件逐字比较一致；导入与新建并列。手机 390px 宽度下检查无横向溢出，页面无脚本异常。', 'small')
    story.append(Image(str(EVIDENCE / 'ui-skills.png'), width=495, height=270))


def footer(canvas, doc):
    canvas.setStrokeColor(colors.HexColor('#DDE8E3'))
    canvas.line(50, 45, A4[0] - 50, 45)
    canvas.setFont('Body', 8)
    canvas.setFillColor(GRAY)
    canvas.drawString(50, 31, 'CampusClaw | 焦阳 | 2026-10-27')
    canvas.drawRightString(A4[0] - 50, 31, str(doc.page))


doc = SimpleDocTemplate(str(TEMP), pagesize=A4, rightMargin=50, leftMargin=50, topMargin=47, bottomMargin=62, title='CampusClaw 个人 AI 应用平台验证报告', author='焦阳')
doc.build(story, onFirstPage=footer, onLaterPages=footer)
writer = PdfWriter()
writer.clone_document_from_reader(PdfReader(str(TEMP)))
complete_evidence = {**DATA, 'catalog': CATALOG, 'source_revision': REVISION or 'uncommitted-worktree'}
for name in ('ui-check', 'seed-check'):
    source = EVIDENCE / (name + '.json')
    if source.exists():
        complete_evidence[name] = json.loads(source.read_text())
evidence_bytes = json.dumps(complete_evidence, ensure_ascii=False, indent=2).encode()
(OUT.parent / 'campusclaw-personal-ai-platform-evidence.json').write_bytes(evidence_bytes)
writer.add_attachment('verification-evidence.json', evidence_bytes)
with OUT.open('wb') as stream:
    writer.write(stream)
reader = PdfReader(str(OUT))
assert 'verification-evidence.json' in reader.attachments
assert len(reader.pages) >= 9
print('PDF:', OUT, 'pages:', len(reader.pages))
