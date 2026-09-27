import assert from 'node:assert/strict'
import { createElement } from 'react'
import { renderToStaticMarkup } from 'react-dom/server'
import ReactMarkdown from 'react-markdown'

const malicious = '# Lesson\n<script>alert(1)</script>\n<img src=x onerror=alert(2)>\n[click](javascript:alert(3))'
const html = renderToStaticMarkup(createElement(ReactMarkdown, { skipHtml: true }, malicious))

assert.match(html, /<h1>Lesson<\/h1>/)
assert.doesNotMatch(html, /<script|<img|onerror|javascript:/i)
console.log('Markdown renders headings without executing raw HTML or javascript: links')
