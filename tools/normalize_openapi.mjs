import { readFile, writeFile } from 'node:fs/promises'
import { createRequire } from 'node:module'

const require = createRequire(new URL('../web/package.json', import.meta.url))
const { parseDocument } = require('yaml')

const path = process.argv[2]
if (!path) {
  throw new Error('usage: normalize_openapi.mjs <openapi.yaml>')
}

const document = parseDocument(await readFile(path, 'utf8'))
const schemePath = ['components', 'securitySchemes', 'CookieAuth']
if (!document.hasIn(schemePath)) {
  throw new Error('CookieAuth security scheme is missing')
}

document.setIn([...schemePath, 'type'], 'apiKey')
document.setIn([...schemePath, 'in'], 'cookie')
document.setIn([...schemePath, 'name'], '__Host-tdns-session')
document.setIn(
  [...schemePath, 'description'],
  'Opaque HttpOnly browser session cookie set by the browser-code exchange or password login.'
)

await writeFile(path, document.toString({ lineWidth: 0 }))
