import React from 'react'
import { createRoot } from 'react-dom/client'
import { Tldraw, createShapeId, PageRecordType, toRichText, serializeTldrawJson, parseTldrawJsonFile } from 'tldraw'
import { getAssetUrls } from '@tldraw/assets/selfHosted'
import 'tldraw/tldraw.css'
const createPageId = name => PageRecordType.createId(name)

// All examples are invented. The generator runs on loopback, not a deployed editor.
function mount(editor) {
  editor.user.updateUserPreferences({ colorScheme: 'light' })
  const originalPage = editor.getCurrentPageId()
  let prefix, shapes
  const id = (name) => createShapeId(`${prefix}-${name}`)
  const text = (name, x, y, value, w = 1000, extra = {}) => shapes.push({
    id: id(name), type: 'text', x, y,
    props: { richText: toRichText(value), font: 'draw', size: 's', w, autoSize: false, ...extra },
  })
  const box = (name, x, y, w, h, color = 'blue', extra = {}) => shapes.push({
    id: id(name), type: 'geo', x, y,
    props: { geo: 'rectangle', w, h, color, fill: 'semi', dash: 'draw', size: 's', ...extra },
  })
  const arrow = (name, x, y, w, color = 'blue') => shapes.push({
    id: id(name), type: 'arrow', x, y,
    props: { start: { x: 0, y: 0 }, end: { x: w, y: 0 }, color, arrowheadEnd: 'arrow', size: 'm' },
  })

  window.drawFrame = async (kind, step, progress) => {
    prefix = `${kind}-${step}-${progress}`
    const page = createPageId(prefix)
    editor.createPage({ id: page, name: `${step + 1}. ${kind} (${progress + 1}/3)` })
    editor.setCurrentPage(page)
    shapes = []
    box('canvas', 0, 0, 1100, 510, 'grey', { fill: 'none', dash: 'solid' })
    text('heading', 32, 20, kind === 'learn' ? 'Learn a template, one line at a time' : 'Train once. Freeze. Match without mutation.', 1040, { size: 'l' })
    text('subtitle', 34, 76, kind === 'learn' ? 'Same input order → same learned state' : 'Freeze is permanent. The matcher shares the learned tree.', 1030, { color: 'grey' })

    if (kind === 'learn') {
      text('input-label', 35, 122, 'ORDERED INPUT', 480, { color: 'blue' })
      const names = ['alice', 'bob', 'carol']
      names.forEach((name, i) => {
        box(`row-${i}`, 34, 165 + i * 73, 475, 59, i === step ? 'blue' : 'grey', { fill: i === step ? 'semi' : 'none' })
        text(`line-${i}`, 51, 180 + i * 73, `user ${name} signed in`, 445, { font: 'mono', color: i <= step ? 'black' : 'grey' })
      })
      arrow('flow', 541, 266, 66)
      box('packet', 540 + progress * 24, 251, 13, 13, 'blue', { geo: 'ellipse', fill: 'solid' })
      text('train', 525, 208, 'Train', 110, { color: 'blue' })
      box('model', 646, 161, 420, 227, 'green')
      const completed = progress === 2 ? step : step - 1
      text('result-label', 665, 178, completed < 0 ? 'WAITING FOR FIRST LINE' : ['Created', 'TemplateChanged', 'Unchanged'][completed], 380, { color: 'green' })
      text('template', 665, 233, completed < 0 ? '(no cluster yet)' : completed === 0 ? 'user alice signed in' : 'user <*> signed in', 382, { font: 'mono' })
      text('count', 665, 308, completed < 0 ? 'Clusters: 0' : `ID: 1     Count: ${completed + 1}`, 380, { font: 'mono' })
      text('footnote', 35, 428, 'Different token → <*>     ·     Training updates counts and templates', 1010, { size: 'm' })
    } else {
      box('miner', 35, 156, 405, 205, step === 0 ? 'green' : 'orange')
      text('miner-title', 55, 173, step === 0 ? 'MUTABLE MINER' : 'FROZEN MINER', 365, { color: step === 0 ? 'green' : 'orange' })
      text('miner-template', 55, 222, 'user <*> signed in', 365, { font: 'mono' })
      text('miner-count', 55, 290, 'ID: 1    Count: 3', 365, { font: 'mono' })
      arrow('freeze-arrow', 475, 255, 111, step === 0 ? 'grey' : 'blue')
      text('freeze-label', 465, 197, 'Freeze()', 145, { color: 'blue', font: 'mono' })
      if (step === 1) box('packet', 480 + progress * 40, 239, 13, 13, 'blue', { geo: 'ellipse', fill: 'solid' })
      box('matcher', 630, 156, 435, 205, step === 0 ? 'grey' : 'blue', { fill: step === 0 ? 'none' : 'semi' })
      text('matcher-title', 650, 173, step === 0 ? 'NOT PUBLISHED YET' : 'READ-ONLY MATCHER', 395, { color: step === 0 ? 'grey' : 'blue' })
      text('matcher-template', 650, 222, step === 0 ? '(train first)' : 'user <*> signed in', 395, { font: 'mono' })
      text('matcher-count', 650, 290, step === 0 ? '' : 'ID: 1    Count: 3', 395, { font: 'mono' })
      if (step === 2) {
        const names = ['dora', 'evan', 'finn']
        names.forEach((name, i) => {
          box(`reader-${i}`, 35 + i * 351, 407, 328, 63, i === progress ? 'blue' : 'grey')
          text(`reader-label-${i}`, 49 + i * 351, 417, `user ${name} signed in\nMatch → ID 1`, 307, { font: 'mono' })
        })
        text('immutable-note', 35, 372, 'Concurrent reads: the count stays 3, and the template stays unchanged.', 1030, { color: 'blue' })
      } else {
        text('freeze-note', 35, 413, step === 0 ? 'Training is allowed until Freeze().' : 'Train → ErrFrozen     ·     No tree copy', 1030, { size: 'm' })
      }
    }
    editor.createShapes(shapes)
    await editor.fonts.loadRequiredFontsForCurrentPage()
    await document.fonts.ready
    const { blob } = await editor.toImage(editor.getCurrentPageShapes(), {
      format: 'png', background: true, padding: 8, scale: 1,
    })
    const bytes = new Uint8Array(await blob.arrayBuffer())
    let binary = ''
    for (const byte of bytes) binary += String.fromCharCode(byte)
    return btoa(binary)
  }
  window.saveDrawing = async (kind) => {
    // Keep the three completed keyframes as editable pages, not the in-between frames.
    for (const page of editor.getPages()) {
      if (page.id !== originalPage && !page.id.endsWith('-2')) editor.deletePage(page.id)
    }
    editor.deletePage(originalPage)
    editor.setCurrentPage(createPageId(`${kind}-2-2`))
    const json = await serializeTldrawJson(editor)
    const parsed = parseTldrawJsonFile({ json, schema: editor.store.schema })
    if (!parsed.ok) throw new Error(`Invalid tldraw round trip: ${JSON.stringify(parsed.error)}`)
    if (parsed.value.allRecords().filter(r => r.typeName === 'page').length !== 3) throw new Error('Expected three editable pages')
    return json
  }
  window.diagramReady = true
}

createRoot(document.getElementById('root')).render(
  <div style={{ position: 'fixed', inset: 0 }}><Tldraw hideUi assetUrls={getAssetUrls({ baseUrl: '/node_modules/@tldraw/assets' })} onMount={mount} /></div>,
)
