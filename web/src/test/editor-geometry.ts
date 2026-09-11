// jsdom has no layout engine. These geometry-only shims let the real CodeMirror
// consume input; they do not assert or simulate rendered layout.
Object.defineProperties(Range.prototype, {
  getClientRects: { configurable: true, value: () => [] },
  getBoundingClientRect: { configurable: true, value: () => new DOMRect() },
});
