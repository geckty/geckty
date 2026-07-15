# Notice

This package is vendored from [`github.com/cfoust/cy`](https://github.com/cfoust/cy)'s
`pkg/emu` (VT/ANSI state machine) and a small subset of its `pkg/geom`
package (only `Vec2`, `Size`, `Rect`, and the integer helpers `emu` itself
uses), pinned at `v1.12.0`.

## Why vendored

`cy/pkg/emu`'s `vt_posix.go` excluded `windows` from its build constraint,
despite the file containing no platform-specific code (confirmed by
inspection — no syscalls, purely portable state-machine logic; the original
`hinshun/vt10x` this was forked from shipped an equivalent Windows-inclusive
file that `cy`'s fork simply didn't carry forward, almost certainly because
`cy` itself, the parent terminal-multiplexer project, only targets POSIX).
This vendored copy removes that build tag (see `vt.go`, formerly
`vt_posix.go`) — no other behavioral changes were made. All import paths
were rewritten from `github.com/cfoust/cy/pkg/...` to
`github.com/geckty/geckty/internal/vt/emu/...`.

## License

`cy` is MIT licensed:

```
MIT License

Copyright (c) 2024 Caleb Foust

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in
all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
THE SOFTWARE.
```
