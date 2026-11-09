# Diagram sources

The two explainers are rendered by **tldraw SDK 5.6.0**, not an imitation drawing library. All names/messages are invented. Each `.tldr` file contains three editable keyframe pages. PNGs show the completed state; GIFs play once for 4.5 seconds and stop there. The README selects PNGs for `prefers-reduced-motion: reduce` and also links them directly.

## Rebuild locally

Node 22.12+ and ffmpeg are required:

```sh
cd docs/visuals
npm ci --ignore-scripts
npx playwright install chromium
npm run render
```

To use an installed Chromium instead, set `CHROME_PATH` to its executable. The renderer binds only to `127.0.0.1` and is a local development tool; no editor or JavaScript bundle is deployed to GitHub. It uses the SDK's image export and file serializer. Do not deploy this editor without the appropriate [tldraw license](https://tldraw.dev/community/license). Its SDK license is distinct from the Go library's Apache-2.0 license; see [the retained SDK license](../../third_party/tldraw-LICENSE.md).

- [Learn animation](../assets/learn.gif) · [static](../assets/learn.png) · [editable source](../assets/learn.tldr)
- [Freeze animation](../assets/freeze.gif) · [static](../assets/freeze.png) · [editable source](../assets/freeze.tldr)

The examples show a real API sequence: train alice, bob, carol into cluster 1/count 3, freeze, then match dora, evan, finn without changing that count. The drawings illustrate behavior, not a concurrency/performance measurement.
