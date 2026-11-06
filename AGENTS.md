改完代码后运行 make lint 和 make test

实现代码之前先检查 https://mygo.egoist.dev/docs 和 https://mygo.egoist.dev/docs/ui ，如果 mygo 已经提供了满足需求的功能或组件，直接使用，不要自己重新实现。
优先复用 mygo 已有的 ui components https://mygo.egoist.dev/docs/ui/
Styling and themes https://mygo.egoist.dev/docs/ui/styling
Animation https://mygo.egoist.dev/docs/ui/drawing
Glass blur: https://mygo.egoist.dev/docs/plugins/glass#styles
代码本身逻辑不复杂、通过代码即可理解时，不写注释。仅在需要解释复杂逻辑、设计原因或非显而易见的约束时添加注释。
性能：mygo 每帧都会重建整个视图，所以每帧的工作量不能随数据变多而增加。长列表用 `ui.List`，不要用 `ui.Scroll` 把所有条目都建出来；正在增长的长文本要切段，不要整段放进一个 `ui.Text`。
