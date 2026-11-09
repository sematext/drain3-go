import { createServer } from 'vite'
import react from '@vitejs/plugin-react'
import { chromium } from 'playwright'
import { mkdir, writeFile, copyFile, mkdtemp } from 'node:fs/promises'
import { tmpdir } from 'node:os'
import { resolve, join } from 'node:path'
import { fileURLToPath } from 'node:url'
import { execFileSync } from 'node:child_process'

const root = fileURLToPath(new URL('.', import.meta.url))
const out = resolve(root, '../assets')
const frames = await mkdtemp(join(tmpdir(), 'drain3-diagrams-'))
await mkdir(out, { recursive: true })
const server = await createServer({ root, plugins: [react()], server: { host: '127.0.0.1', port: 0 } })
let browser
try {
  await server.listen()
  browser = await chromium.launch({ executablePath: process.env.CHROME_PATH || undefined })
  const page = await browser.newPage({ viewport: { width: 1200, height: 720 }, deviceScaleFactor: 1 })
  page.on('pageerror', error => console.error(error))
  for (const kind of ['learn', 'freeze']) {
    await page.goto(server.resolvedUrls.local[0])
    await page.waitForFunction(() => window.diagramReady)
    for (let step = 0; step < 3; step++) {
      for (let progress = 0; progress < 3; progress++) {
        const png = await page.evaluate(([k, s, p]) => window.drawFrame(k, s, p), [kind, step, progress])
        await writeFile(join(frames, `${kind}-${step * 3 + progress}.png`), Buffer.from(png, 'base64'))
      }
    }
    const drawing = await page.evaluate(k => window.saveDrawing(k), kind)
    await writeFile(join(out, `${kind}.tldr`), drawing)
    await copyFile(join(frames, `${kind}-8.png`), join(out, `${kind}.png`))
    // Nine 500ms frames; no loop extension. Stops on the complete, readable diagram.
    execFileSync('ffmpeg', ['-y', '-loglevel', 'error', '-framerate', '2', '-i', join(frames, `${kind}-%d.png`),
      '-filter_complex', '[0:v]split[a][b];[a]palettegen=stats_mode=full[p];[b][p]paletteuse=dither=bayer',
      '-loop', '-1', join(out, `${kind}.gif`)], { stdio: 'inherit' })
  }
  console.log(`Rendered tldraw PNG/GIF/source files in ${out}; frames retained in ${frames}`)
} finally {
  await browser?.close()
  await server.close()
}
