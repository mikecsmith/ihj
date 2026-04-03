# Changelog

## 0.5.0 (2026-04-03)

Version reset from v1.x to v0.x — the v1.x releases were premature for early-stage software.
All v1.x versions have been retracted in go.mod. If you installed via `go install`, run
`go install github.com/mikecsmith/ihj/cmd/ihj@latest` to pick up the latest v0.x release.


### Features

* add 'my' filter and realistic user identity to demo provider ([e15cc3d](https://github.com/mikecsmith/ihj/commit/e15cc3d18c845825368daca789ac87dd576e17ee))
* add apply command with a review diff ui component ([819f1d6](https://github.com/mikecsmith/ihj/commit/819f1d6b5099907f06d415f65d796bcd18ce5b09))
* add apply to local yaml option and fix sync type change bug ([c909ac4](https://github.com/mikecsmith/ihj/commit/c909ac47235899861accaf4459661f3e5ed01e9d))
* add code syntax highlighting and body theme capabilities via glamour ([06240cd](https://github.com/mikecsmith/ihj/commit/06240cdc424c8a24cabe0e4e069a5bce714dcb2f))
* add FieldAssignee and FieldEmail field types for distinct user-field semantics ([89dfe32](https://github.com/mikecsmith/ihj/commit/89dfe328c7848b1726c14e56cc458c15dcd5d3ae))
* add ihj auth login/logout/status commands ([a089c0b](https://github.com/mikecsmith/ihj/commit/a089c0b10b4e50825776dc32c8da11e088981287))
* add internal/auth package for credential storage ([8409073](https://github.com/mikecsmith/ihj/commit/84090730200f67b3c375e6746a8e995667d8992e))
* add priority icon to child issue display in TUI detail view ([0885aab](https://github.com/mikecsmith/ihj/commit/0885aab0c1ac8222d2e22b128012d50eb1d49703))
* add secondary Ctrl bindings for VHS compatibility ([558b76a](https://github.com/mikecsmith/ihj/commit/558b76a838cde28edf43f2db23f1809a6e8a211e))
* add servers config section and credential store integration ([81e56d6](https://github.com/mikecsmith/ihj/commit/81e56d69a6f865b85779d08c0651cede125a535c))
* add sprint:none to remove issues from sprints ([57aef9e](https://github.com/mikecsmith/ihj/commit/57aef9ec0b4984d1a970b3517c5939961622934e))
* add vim mode with modal key routing and KeyMap-driven bindings ([ef2ba31](https://github.com/mikecsmith/ihj/commit/ef2ba31e0325731692d454eab9b1e2f2977e103f))
* add work item type to child issue display in TUI detail view ([35c9cd7](https://github.com/mikecsmith/ihj/commit/35c9cd77747cbc3d009442360c09e27e80261837))
* add workspace switching in TUI ([f19af39](https://github.com/mikecsmith/ihj/commit/f19af3938b104a98477f2d8bc4a105d9279adc4f))
* add configurable shortcuts for default mode ([9eecbc2](https://github.com/mikecsmith/ihj/commit/9eecbc2354730815e20361b23668570f739deae9))
* add focus mode, tab pane toggle, and configurable layout ([9a81f2c](https://github.com/mikecsmith/ihj/commit/9a81f2c3f62ee9407a4a569c16c2f7072f951ec3))
* add show_help_bar config and fix layout chrome calculation ([fcbe502](https://github.com/mikecsmith/ihj/commit/fcbe502244d5497469163fd10fab6efc58ec5b7e))
* bottom help bar + single line contextual search ([52fc12d](https://github.com/mikecsmith/ihj/commit/52fc12d715ef9785d792fed19d337e1aaf95d205))
* export is now in yaml format and injects a yaml-language-server schema ([7aafab1](https://github.com/mikecsmith/ihj/commit/7aafab1552cbc75bedc0d3221b5500eb19152c56))
* improve bootstrap flow with server selection and masked token input ([8778671](https://github.com/mikecsmith/ihj/commit/8778671388c06e4fc64e2f5aba246584953e9bc2))
* improve bootstrap status color inference with heuristic matching ([73ec061](https://github.com/mikecsmith/ihj/commit/73ec0612f53ebf2c572e440ffd1c2145b6d1f6c8))
* improve extract command with CLI piping, flags, and LLM guidance ([5460c23](https://github.com/mikecsmith/ihj/commit/5460c23141e0b7f864cccb0ae68f51e2c7533076))
* make cache TTL configurable per workspace and globally ([133b135](https://github.com/mikecsmith/ihj/commit/133b1355e309c915b32c19af833c891df01e648e))
* make LLM extract guidance configurable ([f98ae71](https://github.com/mikecsmith/ihj/commit/f98ae71e1b9389af63fdc80ab1083c1f7b6083c6))
* popup persistent help ([3d89ded](https://github.com/mikecsmith/ihj/commit/3d89dedc1bb9ff2835640ab7ca6d569b67fa528c))
* scrum/kanban-aware bootstrap, sprint enum, apply workspace flag ([53486f7](https://github.com/mikecsmith/ihj/commit/53486f7da3d0a493f25cb379a491ce7532ab5f38))
* token security improvements ([9ef7400](https://github.com/mikecsmith/ihj/commit/9ef7400f690ace49a99a1837965e971f4b8d9da7))


### Bug Fixes

* &lt;nil&gt; display and spurious diffs for missing manifest fields ([52a7c5d](https://github.com/mikecsmith/ihj/commit/52a7c5d3883599d9f12e8d072e8e7e0d6eb9d563))
* add status message for post-create field updates ([60101ec](https://github.com/mikecsmith/ihj/commit/60101ecb935d580e42a6a2d2ee3129192fa17683))
* auth commands use modeAuth to skip session creation ([bf39af3](https://github.com/mikecsmith/ihj/commit/bf39af3d206674d50b3de51a1f5a2884e6d6bac7))
* background refresh on TUI startup to surface auth errors ([782a4f1](https://github.com/mikecsmith/ihj/commit/782a4f1d570cfbf66d851aa0089c25f56fee5330))
* bootstrap not prompting for server and crashing on empty config ([697db8b](https://github.com/mikecsmith/ihj/commit/697db8bc1eff981ea8f9cb874084ee1982b9ec13))
* consistent Title Case in help overlay key display ([1b950a6](https://github.com/mikecsmith/ihj/commit/1b950a66307beb53ffcaf30affeebcd2d7df6800))
* correct fullscreen layout double-counting outer padding ([aff6d88](https://github.com/mikecsmith/ihj/commit/aff6d88a99deb6936673d1f61d7f22467bbe1a62))
* dim the em dash placeholder for empty assignee/reporter ([b0facae](https://github.com/mikecsmith/ihj/commit/b0facaec1d6339e138b9d31ae41762e876e98066))
* don't quit on Esc in vim normal mode ([b6fa39f](https://github.com/mikecsmith/ihj/commit/b6fa39fd179e66fc506baa80c782f56cc034d55b))
* edit merge now clears ParentID when parent is removed server-side ([c12b71a](https://github.com/mikecsmith/ihj/commit/c12b71ae543485a3dbcf06a65c50fe7a405cae02))
* guard syncDetail against destroying child navigation state ([2f5673e](https://github.com/mikecsmith/ihj/commit/2f5673ed03bc4e898af5461bb03e4af9591520bf))
* handle assignee unassign flow end-to-end with sentinel normalisation ([402174a](https://github.com/mikecsmith/ihj/commit/402174a56f7bac92fdfaf603aed47424e949a1da))
* panic when rendering codeblocks with glamour ([10c9ac7](https://github.com/mikecsmith/ihj/commit/10c9ac77c491535cfd88ee5a7354f415d2ff29ea))
* regression in tui launching post refactor ([71e35fb](https://github.com/mikecsmith/ihj/commit/71e35fb0ebe7f303a34767a3aec4adc2502e0438))
* resolve post-upsert race between transition and fetch ([c40abef](https://github.com/mikecsmith/ihj/commit/c40abeff85f8942239f95a1f448f466d16c8fa78))
* restore fullscreen detail height ([6ef92f1](https://github.com/mikecsmith/ihj/commit/6ef92f1418faa2349b48be2f122ce0a90bfaed3e))
* show help bar in fullscreen mode ([45bfd96](https://github.com/mikecsmith/ihj/commit/45bfd96ed8e174abcb5a11fefe8c9dfd796a62e2))
* smooth scrolling and preserve scroll position after reload ([20e21e5](https://github.com/mikecsmith/ihj/commit/20e21e56f4bdeab214572900667f06796ff1a2c6))
* spurious blank line under footer ([fb2c2b2](https://github.com/mikecsmith/ihj/commit/fb2c2b2c45aeaba356c79921ab3065de88ce6ddb))
