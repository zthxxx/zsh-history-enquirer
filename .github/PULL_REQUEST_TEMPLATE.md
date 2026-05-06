<!-- PR title: conventional-commits prefix
     (feat / fix / chore / docs / refactor / test / ci / build / perf) -->

### Summary

<!-- One paragraph: what this changes and why. -->

### Spec / design impact

<!-- - If this changes user-visible behaviour: which `docs/spec/*.md` did you update?
     - If this changes the Go layering or DI graph: which `docs/design/*.md` did you update?
     - If neither: say so explicitly. -->

### Test plan

- [ ] `task check:fast` passes (fmt + lint + unit)
- [ ] `task test:e2e` passes (debian + alpine, 11 scenarios each)
- [ ] CI green on the PR branch
- [ ] If user-visible: a new e2e scenario asserts the new behaviour
- [ ] If multi-line / wrap related: the new e2e scenario covers
      multi-line entries (the project's most-emphasised boundary)

### Out of scope

<!-- Things you noticed but did not fix in this PR. Linking to a
     follow-up issue is fine. Keeps the PR focused. -->
