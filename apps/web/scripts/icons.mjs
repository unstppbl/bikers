import { readFile, writeFile } from 'node:fs/promises'
import { URL } from 'node:url'
import { Resvg } from '@resvg/resvg-js'

const source = await readFile(new URL('../public/app-icon.svg', import.meta.url), 'utf8')
for (const size of [180, 192, 512]) {
  const renderer = new Resvg(source, { fitTo: { mode: 'width', value: size } })
  await writeFile(new URL(`../public/app-icon-${size}.png`, import.meta.url), renderer.render().asPng())
}
