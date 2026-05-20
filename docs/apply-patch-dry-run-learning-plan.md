# ApplyPatch Dry-Run Learning Plan

这个学习任务的目标不是一次性做完一个“大功能”。目标是通过一个很小的 `applyPatch`
改造，理解 coding agent 写文件时应该如何先检查、再执行、再汇报。

最终效果：

```ts
await applyPatch({
  patch: "...",
  dryRun: true,
});
```

当 `dryRun` 是 `true` 时，工具只解析和校验
patch，返回会影响哪些文件，但不真正写入、 删除或创建文件。

## Why This Matters

`applyPatch` 是 coding agent
里风险最高的工具之一，因为它可以改多个文件、创建文件、
删除文件。现在它已经会先校验路径，再写入文件，这是一个好的开始。

但它还缺少一个重要能力：在真正写入前，告诉用户和 agent 这次 patch 会做什么。

dry-run 模式带来的收益：

- 让 agent 可以先检查 patch 是否有效，再决定是否执行。
- 让 CLI 或未来 UI 可以展示将要修改的文件。
- 为后续 approval flow 做准备，比如删除文件或大范围改动需要用户确认。
- 降低部分写入的风险，尤其是一个 patch 里包含多个操作时。
- 让测试更容易覆盖“验证通过但不写入”的行为。

## Current Flow

先读这些文件：

- `src/tools/apply-patch.ts`
- `src/project-path.ts`
- `src/agent-tool-result.ts`
- `src/errors.ts`
- `tests/apply-patch.test.ts`

当前 `applyPatch` 的流程是：

```text
patch string
-> parseLinePatch
-> validate project paths
-> apply add/delete/update operations
-> return changedFiles
```

dry-run 之后，目标流程是：

```text
patch string
-> parseLinePatch
-> validate project paths
-> validate update hunks
-> if dryRun: return preview
-> apply add/delete/update operations
-> return changedFiles
```

## Step 1: Understand The Patch Operation Types

Read:

- `AddOperation`
- `DeleteOperation`
- `UpdateOperation`
- `PatchOperation`

Learning goal:

理解为什么 `applyPatch` 先把文本 patch 解析成结构化
operation，而不是边读字符串边写文件。

Exercise:

- 找出 add、delete、update 三种操作分别需要哪些文件系统检查。
- 找出 update 操作为什么还需要检查 hunk 是否唯一匹配。

Done when:

- 你能用自己的话解释 `parseLinePatch` 输出的数据结构。
- 你能说清楚 add、delete、update 的风险分别在哪里。

## Step 2: Add A Small API Shape

Change the public input shape from:

```ts
applyPatch(input: { patch: string })
```

to:

```ts
applyPatch(input: { patch: string; dryRun?: boolean })
```

Tool schema also needs:

```ts
dryRun: z.boolean().optional();
```

Learning goal:

理解内部函数 API 和 AI SDK tool schema 是两层不同边界。内部函数服务测试和复用，
tool schema 服务模型调用。

Done when:

- 不传 `dryRun` 时，现有行为不变。
- `createApplyPatchTool` 接受可选 `dryRun`。
- 旧测试不需要大改。

## Step 3: Write The First Dry-Run Test

Add one test first:

```text
dry-run update returns changedFiles without writing the file
```

Test shape:

- 创建 `index.ts`
- 调用 `applyPatch({ patch, dryRun: true })`
- 断言返回 `changedFiles: ["index.ts"]`
- 断言文件内容没有变化

Learning goal:

用测试锁住最小行为：dry-run 是预览，不是写入。

Done when:

- 新测试失败，失败原因是当前代码仍然写入文件。

## Step 4: Separate Validation From Writing

Refactor `applyPatch` into two conceptual phases:

```text
prepare operations
apply operations
```

The first phase should:

- parse patch
- resolve and validate project paths
- check update hunk existence
- check update hunk uniqueness
- build `changedFiles`

The second phase should:

- create files for add operations
- remove files for delete operations
- write updated content for update operations

Learning goal:

理解为什么 safe tools should validate before mutating state。

Done when:

- `dryRun: true` returns after validation.
- `dryRun: false` or omitted still applies changes.
- The original apply tests still pass.

## Step 5: Cover Add And Delete Dry-Run Behavior

Add tests:

```text
dry-run add does not create the file
dry-run delete does not remove the file
```

Learning goal:

确认 dry-run 对所有 operation 类型都是一致的。

Done when:

- add dry-run 返回目标文件，但文件不存在。
- delete dry-run 返回目标文件，但文件仍然存在。

## Step 6: Keep Safety Checks Active In Dry-Run

Add or adjust tests to prove:

```text
dry-run still rejects blocked files
dry-run still rejects paths outside the project
dry-run still rejects missing update hunks
dry-run still rejects ambiguous update hunks
```

Learning goal:

理解 dry-run 不是“轻量模式”或“跳过检查模式”。它只跳过写入，不跳过安全校验。

Done when:

- blocked `.env` still fails.
- `../outside.txt` still fails.
- missing or ambiguous hunks still fail.
- no file is changed after a failed dry-run.

## Step 7: Improve The Return Shape Carefully

Keep the first version small:

```ts
{
  changedFiles: string[];
  dryRun: boolean;
}
```

Do not add a large preview object yet.

Learning goal:

避免一开始就设计过大的 API。先交付一个稳定的小能力。

Done when:

- normal apply returns `dryRun: false`.
- dry-run returns `dryRun: true`.
- tool wrapper preserves the same data shape inside `AgentToolResult`.

## Step 8: Run Checks

Run:

```bash
deno task check
```

If only a narrower test is needed during development:

```bash
deno test --allow-all tests/apply-patch.test.ts
```

Learning goal:

养成每个小行为都有回归测试的习惯。

Done when:

- apply patch tests pass.
- full project check passes.

## Step 9: Discuss The Design

After implementation, review these questions:

- Did we validate every operation before writing any file?
- Can dry-run be used by approval UI later?
- Is the return shape too small, too large, or just enough?
- What would break if a patch has one valid hunk and one invalid hunk?
- Should dry-run include operation types such as add/update/delete in the
  future?

Learning goal:

把一次代码改动升级成 runtime design 思考。

## Suggested Session Order

Use this order when learning with an agent:

1. Read `apply-patch.ts` and explain the current flow.
2. Add the first failing dry-run update test.
3. Make the smallest code change to pass that test.
4. Add add/delete dry-run tests.
5. Refactor validation so update hunks are checked before any write.
6. Add blocked path and failed hunk dry-run tests.
7. Update the tool schema and wrapper test.
8. Run checks and inspect the diff.
9. Summarize what changed and what safety property improved.

## Out Of Scope For This Task

Do not build these in the first version:

- approval UI
- patch size risk scoring
- operation-by-operation preview output
- persistent execution history
- a full git-style patch parser
- WebSocket or frontend display

These are good future tasks, but adding them now would make this lesson too
broad.
