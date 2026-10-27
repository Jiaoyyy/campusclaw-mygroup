// Verify template selection and Markdown import using the actual browser UI.
import { createRequire } from 'node:module'
import { readFileSync, mkdirSync, writeFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import path from 'node:path'
const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..')
const require = createRequire(import.meta.url)
const { chromium } = require(process.env.PLAYWRIGHT_MODULE || '/Users/jiaoyang/.cache/codex-runtimes/codex-primary-runtime/dependencies/node/node_modules/playwright')
const config = Object.fromEntries(readFileSync(path.join(root, '.env'), 'utf8').split('\n').filter(line => line.includes('=') && !line.trim().startsWith('#')).map(line => { const i = line.indexOf('='); return [line.slice(0, i).trim(), line.slice(i + 1).trim().replace(/^['"]|['"]$/g, '')] }))
const out = path.join(root, 'output/assignment-evidence')
mkdirSync(out, { recursive: true })
const browser = await chromium.launch({ channel: 'chrome', headless: true })
try {
  const page = await browser.newPage({ viewport: { width: 1440, height: 1100 }, deviceScaleFactor: 1 })
  const errors = []
  page.on('pageerror', error => errors.push(error.message))
  await page.goto('http://127.0.0.1:18382/login')
  await page.getByLabel('账号', { exact: true }).fill('teacher_a')
  await page.getByLabel('密码', { exact: true }).fill(config.SEED_TEACHER_A_PASSWORD)
  await page.getByRole('button', { name: '登录', exact: true }).click()
  await page.waitForURL('**/materials')
  await page.goto('http://127.0.0.1:18382/tutor?view=settings')
  await page.getByRole('button', { name: '＋ 新建', exact: true }).click()
  const name = '界面验证-知识复习-' + Date.now().toString().slice(-6)
  await page.getByLabel('名称', { exact: true }).fill(name)
  await page.getByLabel('使用提示词模板').selectOption('review')
  const prompt = await page.getByLabel('角色提示词', { exact: true }).inputValue()
  if (!prompt.startsWith('你是本班的知识复习助手')) throw new Error('Template was not filled')
  await page.getByRole('button', { name: '保存智能体', exact: true }).click()
  await page.getByRole('button').filter({ hasText: name }).click()
  await page.getByRole('button', { name: '关闭提示', exact: true }).click()
  await page.screenshot({ path: path.join(out, 'ui-applications.png'), clip: { x: 125, y: 400, width: 1190, height: 650 } })
  await page.getByRole('tab', { name: '技能库', exact: true }).click()
  await page.getByLabel('导入 Markdown 技能文件').setInputFiles(path.join(root, 'examples/week05/skills.md'))
  const text = await page.getByLabel('skills.md 完整内容').inputValue()
  if (text !== readFileSync(path.join(root, 'examples/week05/skills.md'), 'utf8')) throw new Error('Imported Markdown differs')
  await page.getByRole('button', { name: '关闭提示', exact: true }).click()
  await page.screenshot({ path: path.join(out, 'ui-skills.png'), clip: { x: 125, y: 400, width: 1190, height: 650 } })
  await page.setViewportSize({ width: 390, height: 844 })
  const overflow = await page.evaluate(() => document.documentElement.scrollWidth > innerWidth)
  if (overflow) throw new Error('Mobile page overflows viewport')
  await page.screenshot({ path: path.join(out, 'ui-mobile.png'), fullPage: true })
  if (errors.length) throw new Error(errors.join('; '))
  writeFileSync(path.join(out, 'ui-check.json'), JSON.stringify({ template_selection: true, markdown_import_exact: true, mobile_no_overflow: true, page_errors: errors }, null, 2))
  console.log('UI template selection, complete Markdown import and mobile layout PASS')
} finally {
  await browser.close()
}
