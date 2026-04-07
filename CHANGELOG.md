# Changelog

## [0.5.15](https://github.com/mikecsmith/ihj/compare/v0.5.14...v0.5.15) (2026-04-07)


### Features

* add filter to extract cli flags ([7ae064e](https://github.com/mikecsmith/ihj/commit/7ae064e2352f279913b6f1daf72ee60b5c345768))
* qall and quitall in vim mode ([f4db90f](https://github.com/mikecsmith/ihj/commit/f4db90f00999e5ca89418f46afcf41c9072caf22))
* qall and quitall in vim mode, metadata table spacing ([ae005be](https://github.com/mikecsmith/ihj/commit/ae005bea450daf59e7be40b2fb3afe010832b5ed))


### Bug Fixes

* extract defaulting to active filter ([b2d5a58](https://github.com/mikecsmith/ihj/commit/b2d5a585eefc00d06360b6317896b8941ad69513))
* extract defaulting to active filter ([abdef5e](https://github.com/mikecsmith/ihj/commit/abdef5ec3793a906840d6b4f0378ecfb3cfb6923))
* inconsistent metadata spacing for arrays ([f4eee5f](https://github.com/mikecsmith/ihj/commit/f4eee5f81fc4f0e29d309aaf475c9ab458bb5572))

## [0.5.14](https://github.com/mikecsmith/ihj/compare/v0.5.13...v0.5.14) (2026-04-07)


### Bug Fixes

* fixed width lists based on longest content ([afe91bb](https://github.com/mikecsmith/ihj/commit/afe91bb9eb5c7d090cdb549339a826f9b54965ce))
* fixed width lists based on longest content ([14f9c7c](https://github.com/mikecsmith/ihj/commit/14f9c7c754a302f95aa6c5842ca4d9162a87251c))

## [0.5.13](https://github.com/mikecsmith/ihj/compare/v0.5.12...v0.5.13) (2026-04-07)


### Bug Fixes

* fall back to debug.ReadBuildInfo for version on go install ([2cb7dc3](https://github.com/mikecsmith/ihj/commit/2cb7dc366f017674ef5d83c0a2917ae0c4e5261b))
* **jira:** include _id suffixed field aliases in JQL validation ([0861a67](https://github.com/mikecsmith/ihj/commit/0861a671f3cdfe1872ce9508cb230f50e1136425))
* version number via go install ([1d977ef](https://github.com/mikecsmith/ihj/commit/1d977ef0b75eb97f45955f8e7b8150335fe104ae))

## [0.5.12](https://github.com/mikecsmith/ihj/compare/v0.5.11...v0.5.12) (2026-04-07)


### Bug Fixes

* **jira:** remove stderr write during cache refresh ([8ec6b1e](https://github.com/mikecsmith/ihj/commit/8ec6b1e728d18e7fa399ebee58502deb427e3dfe))

## [0.5.11](https://github.com/mikecsmith/ihj/compare/v0.5.10...v0.5.11) (2026-04-06)


### Bug Fixes

* empty rows in detail header & summary styling ([a9cb4ad](https://github.com/mikecsmith/ihj/commit/a9cb4ad563b2e9b1ba7a3876c817bfee7dc021ef))
* simplify masonry collapse to fill all gaps deterministically ([243a15d](https://github.com/mikecsmith/ihj/commit/243a15dde2262dc12dd70506d43ffe2ac819a2ee))
* summary matches glamour styling for h1 ([7c0859c](https://github.com/mikecsmith/ihj/commit/7c0859cc70366152fd811859078af3a20bda0fdc))

## [0.5.10](https://github.com/mikecsmith/ihj/compare/v0.5.9...v0.5.10) (2026-04-06)


### Bug Fixes

* all fields in manifest schema ([542c66d](https://github.com/mikecsmith/ihj/commit/542c66d1e044541d83cf63f18011355ae463cca7))
* deduplicate createmeta fields by FieldID ([4e8da29](https://github.com/mikecsmith/ihj/commit/4e8da2913b583707e6c12b6e56947d429698de92))
* include all createmeta fields in FieldDefs and schemas ([255ebb1](https://github.com/mikecsmith/ihj/commit/255ebb17e4fddd9079d69512f9f7cc69f1e2dcdf))
* include all eligible fields in generated schemas ([a5c3960](https://github.com/mikecsmith/ihj/commit/a5c3960bed6454305b767346d41e5bf39fbe0d6c))
* not caching custom values ([136fddc](https://github.com/mikecsmith/ihj/commit/136fddc3c611cb78a3e12f3538bb76d9c66e6e24))
* pin workspace level field aliases ([b5039c8](https://github.com/mikecsmith/ihj/commit/b5039c8c26c0e7368b7d8f10a0a7891baefc5109))
* pin workspace level fields ([d1c420f](https://github.com/mikecsmith/ihj/commit/d1c420f53924839e4ca2c7bef114b3777f4ec674))
* properly handle atlassian custom types ([87533e5](https://github.com/mikecsmith/ihj/commit/87533e5c5e4502b08af7814d9f2daf3b9bcb71af))
* render RichText as markdown in apply diff display ([39ad258](https://github.com/mikecsmith/ihj/commit/39ad258374b6f39be870093d1fd0017232b17d65))
* tui showing fields from the wrong type ([fb2d898](https://github.com/mikecsmith/ihj/commit/fb2d89842e5b2bb0d011f2a7a0dee3f13cb9fcf4))

## [0.5.9](https://github.com/mikecsmith/ihj/compare/v0.5.8...v0.5.9) (2026-04-06)


### Features

* add --version flag with goreleaser ldflags injection ([982dfc9](https://github.com/mikecsmith/ihj/commit/982dfc96484c4c29d6c50bdf8d480c20730bcb44))
* add a jira server which allows demos to use the real Jira provider ([58ea698](https://github.com/mikecsmith/ihj/commit/58ea6988460d0dc0767fdec9e347cbc79616f998))
* add custom fields section to TUI detail pane with demo showcase ([5e315f4](https://github.com/mikecsmith/ihj/commit/5e315f40c055eae78dac97dd164302cbe022bee7))
* add rich text custom field support ([bb846c6](https://github.com/mikecsmith/ihj/commit/bb846c6ca6bcedbac03b0f31b83314b992d6cee0))
* add rich text fields to jira provider ([ff711e5](https://github.com/mikecsmith/ihj/commit/ff711e5e0068f3267f5fcda2e8d373e6fe691511))
* add SetKeys, ComputeChanges, ComputeStateHash primitives to core ([6cc893f](https://github.com/mikecsmith/ihj/commit/6cc893fefbb8ea24ae5df240479cf4628e6b1d0c))
* cache per type metadata for the jira provider ([79884a3](https://github.com/mikecsmith/ihj/commit/79884a3d4823528acbe9888faf81747ba21995eb))
* dynamic cli flags in prep for multiple providers ([7c6df3e](https://github.com/mikecsmith/ihj/commit/7c6df3e206368489a01b61e311b959d550402a83))
* dynamic CLI flags in prep for multiple providers ([5f19f66](https://github.com/mikecsmith/ihj/commit/5f19f667fd66997c18cc96c7907defc61fa7c8e0))
* dynamic custom field discovery inc tui display and informational export convention ([a698b93](https://github.com/mikecsmith/ihj/commit/a698b932eda2aaf2fa80883b493c5890bb7a966a))
* dynamic custom fields for Jira with appropriate caching and display updates ([5a9b0ac](https://github.com/mikecsmith/ihj/commit/5a9b0acef839b622e2bebc5c6718276ea240c401))
* dynamic custom fields in header with responsive layout ([5b6d99c](https://github.com/mikecsmith/ihj/commit/5b6d99c2edc218824f2b5ea351b73bc38700123a))
* natural-order ID sort in tree ([5bf7a6b](https://github.com/mikecsmith/ihj/commit/5bf7a6bac9ad7be939c9245f1766abbd86adef95))
* optional short display name for workspace types ([ea09c80](https://github.com/mikecsmith/ihj/commit/ea09c8042c1e0a8593f34c9f7af0d5eb8ac06e5d))
* search results no longer show tree glyphs as results are fuzzy and breaks ordering ([05fb1db](https://github.com/mikecsmith/ihj/commit/05fb1db7598d82bdb46bac37b3f1c68b9db49f39))
* templated Workspace.BrowseURL ([1aab055](https://github.com/mikecsmith/ihj/commit/1aab055b7e8115b403cf27b87fc13886ab232252))
* track decoded keys in manifest for clear-intent parity with frontmatter ([ac732b1](https://github.com/mikecsmith/ihj/commit/ac732b1dbf8a013dd3dd6a1ec86aba228de2c089))
* tui improvements post dynamic jira fields ([249c306](https://github.com/mikecsmith/ihj/commit/249c3067c186fd5584d14d8b9c303a2e98e989c2))
* wire custom field extraction and deprecate custom_fields in config ([8cc3d79](https://github.com/mikecsmith/ihj/commit/8cc3d7976e4394f3c17bee14c66ddca315e7dad7))
* WorkItem display hints (DisplayID, Location, Indicators) ([8f9a31c](https://github.com/mikecsmith/ihj/commit/8f9a31c4febc75a942bf53a17ae090d54328d6ad))


### Bug Fixes

* _created and _updated date time formats in full export ([638491b](https://github.com/mikecsmith/ihj/commit/638491b886398c345dfd6786a1ab26a0d5734231))
* additionally fixed rich text fields not handling correctly in ([f726933](https://github.com/mikecsmith/ihj/commit/f72693300bf602752de4468c8709989fff4c892a))
* ast issues with checkboxes and lists not preserving properly during conversion between formats ([f80c96f](https://github.com/mikecsmith/ihj/commit/f80c96fd92d63fa5c7f46c1f4a63bfa445bbb884))
* ast issues with checkboxes and lists not preserving properly during conversion between formats ([eee2080](https://github.com/mikecsmith/ihj/commit/eee2080804b56c8f4bbd7fddf28ecd253d8a7530))
* created appearing in default export ([8e07ec8](https://github.com/mikecsmith/ihj/commit/8e07ec8a22cb271869a81cf76df03a58c13c65b6))
* demo mode needs its own temp cache now using real provider ([b3456a6](https://github.com/mikecsmith/ihj/commit/b3456a64a85e19d51f68509e5bc82c48149e3042))
* empty ADF listItem must contain a paragraph child ([da276fa](https://github.com/mikecsmith/ihj/commit/da276fa835145cb99e38af178fec774767692295))
* exclude informational fields from standard export properly ([125ea2c](https://github.com/mikecsmith/ihj/commit/125ea2cafada45290cf805a323004b66ddc3eef6))
* make updated immutable for Jira ([e65adff](https://github.com/mikecsmith/ihj/commit/e65adff34d0504de075192c03922d26ad6b77708))
* new concurrency issue after sync once implementation ([4fb9816](https://github.com/mikecsmith/ihj/commit/4fb9816d3e4e7bc69bba30ffb675d544d13dab92))
* omit description from Jira create payload when nil ([9ddf6ed](https://github.com/mikecsmith/ihj/commit/9ddf6ed6e8056c3c09cc0b4c7b397dd3e54f486f))
* properly indent yaml export with list items ([8c62844](https://github.com/mikecsmith/ihj/commit/8c62844e5fe1ee10530ee34f674367a8853bd594))
* regression in headless envs with keychain now demo provider is gone ([bd6df18](https://github.com/mikecsmith/ihj/commit/bd6df18c13188c0a3ca6a2de4f8a52457da3f481))
* regression with child list spacing ([dfd8477](https://github.com/mikecsmith/ihj/commit/dfd8477e014c85986b004f00d46bed0206053ad9))
* resolve potential deadlock in tui ui bridge ([71fe3d1](https://github.com/mikecsmith/ihj/commit/71fe3d1c8a7d54c2ac753a8329559df0f4c4fc5c))
* resolve potential deadlock in tui ui bridge ([0041671](https://github.com/mikecsmith/ihj/commit/004167180e595eda3238b615e15b129491611bb7))
* run goreleaser from release-please workflow ([29a4987](https://github.com/mikecsmith/ihj/commit/29a4987bd82bd77c450ad4c6ca04ac5b68373097))
* run goreleaser from release-please workflow ([8a16fd2](https://github.com/mikecsmith/ihj/commit/8a16fd29f652ebbd12422f9c653f43481e09cbaa))
* sprint action in applys diff view ([02f8caa](https://github.com/mikecsmith/ihj/commit/02f8caa377fef6c977b425035e98d228fec40543))
* start hint keys at 1 not 0 ([966fc3a](https://github.com/mikecsmith/ihj/commit/966fc3a8424b8d0fbc3585acb2984e70f724a1b7))

## [0.5.8](https://github.com/mikecsmith/ihj/compare/v0.5.7...v0.5.8) (2026-04-05)


### Features

* add rich text custom field support ([bb846c6](https://github.com/mikecsmith/ihj/commit/bb846c6ca6bcedbac03b0f31b83314b992d6cee0))
* add rich text fields to jira provider ([ff711e5](https://github.com/mikecsmith/ihj/commit/ff711e5e0068f3267f5fcda2e8d373e6fe691511))


### Bug Fixes

* demo mode needs its own temp cache now using real provider ([b3456a6](https://github.com/mikecsmith/ihj/commit/b3456a64a85e19d51f68509e5bc82c48149e3042))

## [0.5.7](https://github.com/mikecsmith/ihj/compare/v0.5.6...v0.5.7) (2026-04-05)


### Features

* add a jira server which allows demos to use the real Jira provider ([58ea698](https://github.com/mikecsmith/ihj/commit/58ea6988460d0dc0767fdec9e347cbc79616f998))


### Bug Fixes

* regression in headless envs with keychain now demo provider is gone ([bd6df18](https://github.com/mikecsmith/ihj/commit/bd6df18c13188c0a3ca6a2de4f8a52457da3f481))

## [0.5.6](https://github.com/mikecsmith/ihj/compare/v0.5.5...v0.5.6) (2026-04-05)


### Features

* dynamic custom fields in header with responsive layout ([5b6d99c](https://github.com/mikecsmith/ihj/commit/5b6d99c2edc218824f2b5ea351b73bc38700123a))
* search results no longer show tree glyphs as results are fuzzy and breaks ordering ([05fb1db](https://github.com/mikecsmith/ihj/commit/05fb1db7598d82bdb46bac37b3f1c68b9db49f39))
* tui improvements post dynamic jira fields ([249c306](https://github.com/mikecsmith/ihj/commit/249c3067c186fd5584d14d8b9c303a2e98e989c2))


### Bug Fixes

* new concurrency issue after sync once implementation ([4fb9816](https://github.com/mikecsmith/ihj/commit/4fb9816d3e4e7bc69bba30ffb675d544d13dab92))
* regression with child list spacing ([dfd8477](https://github.com/mikecsmith/ihj/commit/dfd8477e014c85986b004f00d46bed0206053ad9))
* start hint keys at 1 not 0 ([966fc3a](https://github.com/mikecsmith/ihj/commit/966fc3a8424b8d0fbc3585acb2984e70f724a1b7))

## [0.5.5](https://github.com/mikecsmith/ihj/compare/v0.5.4...v0.5.5) (2026-04-04)


### Bug Fixes

* ast issues with checkboxes and lists not preserving properly during conversion between formats ([f80c96f](https://github.com/mikecsmith/ihj/commit/f80c96fd92d63fa5c7f46c1f4a63bfa445bbb884))
* ast issues with checkboxes and lists not preserving properly during conversion between formats ([eee2080](https://github.com/mikecsmith/ihj/commit/eee2080804b56c8f4bbd7fddf28ecd253d8a7530))
* run goreleaser from release-please workflow ([29a4987](https://github.com/mikecsmith/ihj/commit/29a4987bd82bd77c450ad4c6ca04ac5b68373097))
* run goreleaser from release-please workflow ([8a16fd2](https://github.com/mikecsmith/ihj/commit/8a16fd29f652ebbd12422f9c653f43481e09cbaa))

## [0.5.4](https://github.com/mikecsmith/ihj/compare/v0.5.3...v0.5.4) (2026-04-04)


### Features

* dynamic custom fields for Jira with appropriate caching and display updates ([5a9b0ac](https://github.com/mikecsmith/ihj/commit/5a9b0acef839b622e2bebc5c6718276ea240c401))

## [0.5.3](https://github.com/mikecsmith/ihj/compare/v0.5.2...v0.5.3) (2026-04-04)


### Bug Fixes

* resolve potential deadlock in tui ui bridge ([71fe3d1](https://github.com/mikecsmith/ihj/commit/71fe3d1c8a7d54c2ac753a8329559df0f4c4fc5c))
* resolve potential deadlock in tui ui bridge ([0041671](https://github.com/mikecsmith/ihj/commit/004167180e595eda3238b615e15b129491611bb7))

## [0.5.2](https://github.com/mikecsmith/ihj/compare/v0.5.1...v0.5.2) (2026-04-03)


### Features

* add --version flag with goreleaser ldflags injection ([982dfc9](https://github.com/mikecsmith/ihj/commit/982dfc96484c4c29d6c50bdf8d480c20730bcb44))
* dynamic cli flags in prep for multiple providers ([7c6df3e](https://github.com/mikecsmith/ihj/commit/7c6df3e206368489a01b61e311b959d550402a83))
* dynamic CLI flags in prep for multiple providers ([5f19f66](https://github.com/mikecsmith/ihj/commit/5f19f667fd66997c18cc96c7907defc61fa7c8e0))

## [0.5.1](https://github.com/mikecsmith/ihj/compare/v0.5.0...v0.5.1) (2026-04-03)


### Bug Fixes

* empty ADF listItem must contain a paragraph child ([da276fa](https://github.com/mikecsmith/ihj/commit/da276fa835145cb99e38af178fec774767692295))
* omit description from Jira create payload when nil ([9ddf6ed](https://github.com/mikecsmith/ihj/commit/9ddf6ed6e8056c3c09cc0b4c7b397dd3e54f486f))

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
