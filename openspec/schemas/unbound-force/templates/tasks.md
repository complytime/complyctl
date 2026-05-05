<!--
  [P] marks tasks eligible for parallel execution.
  Add [P] when a task: (a) touches different files from
  other [P] tasks in the group, (b) has no dependency
  on prior tasks in the group, (c) can safely execute
  without ordering constraints.
  Do NOT add [P] when tasks modify the same file —
  parallel workers will cause merge conflicts.
  Tasks without [P] run sequentially first, then [P]
  tasks run in parallel.
-->

## 1. <!-- Task Group -->

- [ ] 1.1 <!-- sequential task (runs first) -->
- [ ] 1.2 [P] <!-- parallel task (different file) -->
- [ ] 1.3 [P] <!-- parallel task (different file) -->

## 2. <!-- Task Group -->

- [ ] 2.1 <!-- task description -->
<!-- scaffolded by uf vdev -->
