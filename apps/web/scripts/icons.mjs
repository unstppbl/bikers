import { readFile, writeFile } from 'node:fs/promises'
import { URL } from 'node:url'
import { Resvg } from '@resvg/resvg-js'

// app-icon.svg holds outlined glyphs, so rendering needs no fonts.
// The Apple touch and maskable icons fill the rounded corners with the ground colour,
// because iOS and Android apply their own masks.
const ground = '#131211'
const icons = [
  { file: 'app-icon-192.png', size: 192 },
  { file: 'app-icon-512.png', size: 512 },
  { file: 'app-icon-180.png', size: 180, background: ground },
  { file: 'app-icon-maskable-512.png', size: 512, background: ground },
]

const source = await readFile(new URL('../public/app-icon.svg', import.meta.url), 'utf8')
for (const { file, size, background } of icons) {
  const renderer = new Resvg(source, { fitTo: { mode: 'width', value: size }, background, font: { loadSystemFonts: false } })
  await writeFile(new URL(`../public/${file}`, import.meta.url), renderer.render().asPng())
}
