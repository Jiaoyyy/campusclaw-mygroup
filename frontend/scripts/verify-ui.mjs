import assert from 'node:assert/strict'
import { mkdtemp, rm } from 'node:fs/promises'
import { join } from 'node:path'
import { pathToFileURL } from 'node:url'
import { createElement } from 'react'
import { renderToStaticMarkup } from 'react-dom/server'
import { build } from 'esbuild'

const directory = await mkdtemp(join(process.cwd(), 'node_modules', '.campusclaw-ui-'))
try {
  const output = join(directory, 'page.mjs')
  await build({ entryPoints: ['src/pages/MaterialsPage.tsx'], outfile: output, bundle: true, platform: 'node', format: 'esm', packages: 'external' })
  const { default: MaterialsPage } = await import(pathToFileURL(output).href)
  const profile = { id: 1, username: 'student_a1', role: 'student', class_id: 1, class_name: 'A', csrf_token: 'test' }
  const student = renderToStaticMarkup(createElement(MaterialsPage, { profile, onAuthLost() {} }))
  const teacher = renderToStaticMarkup(createElement(MaterialsPage, { profile: { ...profile, role: 'teacher' }, onAuthLost() {} }))
  assert.doesNotMatch(student, /上传到本班|选择 TXT 或 Markdown 文件/)
  assert.match(teacher, /上传到本班/)
  assert.match(student, /A 班/)
  assert.match(student, /本班知识库检索/)
  assert.match(teacher, /本班知识库检索/)
  assert.match(student, /关键词（当前可用）/)
  assert.doesNotMatch(student, /简短回答|<option value="vector"|混合（默认）/)
  assert.doesNotMatch(student, /按当前策略重建索引/)
  assert.match(teacher, /切分策略/)
  console.log('Keyword-only mode is usable; only teachers can upload and choose chunking')
} finally {
  await rm(directory, { recursive: true, force: true })
}
