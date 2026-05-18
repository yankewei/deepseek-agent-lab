# TODO

这个文件记录当前项目里刻意先做简化版、后面可以继续补上的能力。它不是发布计划，也不是所有任务都必须做；更像学习 coding agent 时的下一步清单。

## Runtime 和执行状态

- [x] 把 Execution State Tracking 从命令执行扩展到所有 tool。
  现在已覆盖所有 AI SDK tool wrapper。`runCommand` 保留 policy 和 approval 的细状态，其他工具使用 `created -> running -> completed / failed`。

- [x] 为 execution record 增加更完整的时间信息。
  目前已有 `startedAt`、`completedAt`、`durationMs` 和 `history`。后面可以继续补每个阶段耗时、失败阶段等字段，方便做调试和性能分析。

- [ ] 做持久化的 execution history。
  当前记录只在内存里。后面可以写入本地 JSONL、SQLite 或项目内 `.agent/` 目录，用来支持历史回放、恢复和排查问题。

- [ ] 支持 agent resume。
  现在进程退出后状态就丢了。后面可以基于持久化记录恢复未完成的任务，尤其是等待审批、长命令执行、patch 应用这类流程。

## Event Stream

- [ ] 把事件流从回调升级为可订阅接口。
  现在是 `createExecutionTracker({ onEvent })`。后面可以做 `subscribe` / `unsubscribe`，支持多个订阅者，而不是只能传一个回调。

- [ ] 增加更多事件类型。
  当前只有 `execution_state_changed`。后面可以拆成 `tool_call_started`、`approval_requested`、`approval_resolved`、`command_started`、`command_finished`、`tool_error` 等事件。

- [x] 给事件增加稳定的序号。
  事件现在带有递增的 `sequence`，方便 UI、日志系统和测试按顺序消费事件。

- [ ] 增加事件输出适配器。
  CLI 现在直接 `console.log`。后面可以做 console adapter、JSON adapter、WebSocket adapter，让不同应用层以不同形式消费事件。

## Approval UI 和 Human-in-the-loop

- [x] 支持更丰富的审批决策。
  现在支持 approve once、always allow this command prefix 和 deny。后面可以继续增加 deny and remember 等选项。

- [x] 把审批结果建模成对象。
  `ApprovalPrompt` 现在返回 `{ decision, reason?, policyAmendment? }`，可以表达更丰富的用户选择。

- [x] 根据策略生成真实 risk level。
  command policy 现在会为 prompt decision 输出 `riskLevel`，approval UI 只负责展示。

- [ ] 支持审批超时和取消。
  当前 CLI 会一直等用户输入。后面可以增加 timeout、abort signal 和默认拒绝策略。

- [ ] 做更正式的审批 UI。
  现在只是 CLI 文本。后面如果做 Web UI，可以基于同一个 `ApprovalRequest` 渲染审批卡片、按钮和历史记录。

## Policy Engine

- [ ] 把命令策略从硬编码升级为规则配置。
  现在 allow / prompt / forbidden 都写在 `policy.ts` 里。后面可以支持本地规则文件，比如 `agent-policy.json` 或 `.agent/rules.json`。

- [x] 支持 prefix rule。
  现在支持进程内 command prefix rule，用户批准某类命令后可在当前运行中复用规则。

- [ ] 增加 granular approval mode。
  当前没有全局审批模式。后面可以支持 `never`、`on-request`、`unless-trusted`、`granular` 这类模式，用来控制哪些请求能弹审批、哪些必须直接拒绝。

- [x] 为统一策略层做类型命名准备。
  已把 command-only 的 policy decision 类型抽成通用 `PolicyDecision<Code, Subject>`，`CommandPolicyDecision` 作为命令策略特化类型存在，保持行为不变，为后续文件、patch、网络策略迁移铺路。

- [ ] 增加文件、patch、网络的统一策略判断。
  现在命令有 policy，文件路径和 patch 主要靠各自工具内部校验。后面可以抽成统一策略层，让每类动作都返回 allow / prompt / forbidden。

- [x] 给 policy decision 增加 machine-readable code。
  policy decision 现在同时包含 `type`、`code` 和 `reason`，例如 `COMMAND_NOT_ALLOWED`、`SHELL_OPERATOR_BLOCKED`、`DEPENDENCY_CHANGE_REQUIRES_APPROVAL`。

## AgentToolResult 和错误分类

- [x] 把 `AgentToolResult` 推广到所有工具。
  所有 AI SDK tool wrapper 都返回 `{ ok, data, error, meta }`。内部业务函数仍保留原始返回值，方便测试和复用。

- [ ] 建立正式的 Error Taxonomy。
  目前只有 `POLICY_FORBIDDEN`、`APPROVAL_REASON_REQUIRED`、`EXECUTION_FAILED`。后面可以扩展为 `VALIDATION_FAILED`、`PATH_OUTSIDE_PROJECT`、`PATCH_APPLY_FAILED`、`APPROVAL_DENIED` 等。

- [ ] 区分 tool error 和业务失败。
  有些情况应该作为 `{ ok: false }` 返回给模型，有些才应该真正 throw。后面需要明确边界，避免工具错误处理混乱。

- [ ] 给 envelope 增加用户可读摘要。
  后面可以在 `meta` 或单独字段里加 `summary`，让 CLI / UI 不用自己拼展示文案。

## 文件编辑和 Patch

- [ ] 改进 patch parser。
  当前 `applyPatch` 只支持一个很小的 patch 子集。后面可以支持更多上下文格式、更好的错误定位和更清晰的失败信息。

- [ ] 给 `applyPatch` 增加预览模式。
  先解析和校验 patch，返回会改哪些文件、哪些行、是否需要审批，再决定是否真正写入。

- [ ] 为写操作接入审批。
  目前写工具主要靠路径限制。后面可以让大范围 patch、删除文件、修改关键配置时进入审批流。

- [ ] 增加更细的写入策略。
  现在按路径阻止 `.env`、`.git`、`node_modules` 等。后面可以支持按文件类型、改动规模、目录范围决定 allow / prompt / forbidden。

## 工具和 Agent Loop

- [ ] 给每个 tool 增加更清晰的描述和输入约束。
  现在 tools 已经能用，但 prompt 还可以更明确地告诉模型什么时候用哪个工具、不要怎么用。

- [ ] 增加 tool call summary。
  每轮结束时可以汇总调用了哪些工具、改了哪些文件、跑了哪些检查、有哪些失败。

- [ ] 把 event stream 接入最终输出。
  当前事件只打印到 CLI。后面可以让最终回答引用 executionId 或关键事件，方便用户追踪。

- [ ] 更明确地判断 agent 什么时候结束。
  现在主要依赖 AI SDK 的 stream 和 step limit。后面可以结合 tool 状态、pending approval、pending execution 来判断是否能结束这一轮。

## Git 和项目工作流

- [ ] 增加 git status / diff / commit 工具。
  现在有 `getDiff`，但 commit 和 push 还没有正式 tool。后面可以把 git 工作流也纳入 policy、approval 和 event stream。

- [ ] 做提交前检查流程。
  比如固定执行 `pnpm check`、查看 diff、生成 commit message、必要时请求用户确认。

- [ ] 支持 PR workflow。
  后面可以增加创建分支、推送、打开 PR、生成 PR 描述、跟踪 CI 状态等能力。

## 测试和质量

- [ ] 增加 end-to-end 测试。
  目前多数是单元测试。后面可以跑一次完整 agent loop，验证 tool call、event stream、approval、result envelope 的组合行为。

- [ ] 增加回归测试夹具。
  对安全限制、路径逃逸、审批拒绝、patch 失败等场景保留固定测试用例。

- [ ] 增加测试覆盖率检查。
  现在没有覆盖率门槛。后面可以用 Vitest coverage 看核心 runtime 的覆盖情况。

- [ ] 增加安全审计清单。
  对命令执行、路径解析、symlink、防止读取密钥、审批绕过等点做一份固定 review checklist。

## 文档和学习材料

- [x] 把学习路线整理成章节化文档。
  README 现在是项目说明。后面可以单独写 `docs/learning-path.md`，按 Basic Agent Loop、Tools、Policy、Approval、State、Events、Results 展开。

- [ ] 给每个核心模块补架构说明。
  可以写 `docs/runtime.md`，解释 `policy.ts`、`command-executor.ts`、`execution-state.ts`、`approval.ts`、`agent-tool-result.ts` 的关系。

- [ ] 增加本地演示命令。
  比如哪些命令能看到 allow、prompt、forbidden、event stream、approval denied、AgentToolResult envelope。
