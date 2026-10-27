"""Run each URL/method case and write a local HTML report (gitignored)."""
import html
import runpy
import sys
from datetime import datetime
from pathlib import Path
from common import Context

HERE = Path(__file__).resolve().parent
CASES = [
    'cases/api/login/POST.py',
    'cases/api/materials/POST.py',
    'cases/api/retrieval/GET.py',
    'cases/api/skills/POST.py',
    'cases/api/agents/POST.py',
    'cases/api/mcp/tools/GET.py',
    'cases/api/conversations/POST.py',
    'cases/api/conversations/id/messages/POST.py',
    'cases/openapi/GET.py',
]


def main():
    context = Context()
    results = []
    for path in CASES:
        try:
            runpy.run_path(str(HERE / path))['run'](context)
            results.append((path, True, '通过'))
            print('PASS', path)
        except Exception as error:
            results.append((path, False, str(error)))
            print('FAIL', path, str(error))
    rows = ''.join(f'<tr><td>{html.escape(path)}</td><td class="{"ok" if success else "fail"}">{"通过" if success else "失败"}</td><td>{html.escape(detail)}</td></tr>' for path, success, detail in results)
    report = f'<!doctype html><html lang="zh-CN"><meta charset="utf-8"><title>CampusClaw 接口测试报告</title><style>body{{font:16px system-ui;margin:3rem;max-width:1100px}}table{{border-collapse:collapse;width:100%}}th,td{{border:1px solid #ddd;padding:.7rem;text-align:left}}.ok{{color:green}}.fail{{color:#b00}}</style><h1>第 5 课接口测试</h1><p>生成时间：{datetime.now().isoformat(timespec="seconds")}</p><table><tr><th>接口脚本</th><th>结果</th><th>说明</th></tr>{rows}</table></html>'
    (HERE / 'report.html').write_text(report)
    print('HTML report:', HERE / 'report.html')
    return 0 if all(success for _, success, _ in results) else 1


if __name__ == '__main__':
    sys.exit(main())
