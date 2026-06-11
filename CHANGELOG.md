# Changelog

## [0.7.0](https://github.com/jjfantini/pindex/compare/v0.6.0...v0.7.0) (2026-06-11)


### Features

* **exportout:** count SEDC as adjusted-correct in benchmark scoring ([67fc2c6](https://github.com/jjfantini/pindex/commit/67fc2c6c694844e5fe926107111f5d2843cc052f))
* **exportout:** SEDC adjusted-correct + haiku adjudications ([1034375](https://github.com/jjfantini/pindex/commit/103437589b602d99760b5ef48cf6dae41c3cf00e))

## [0.6.0](https://github.com/jjfantini/pindex/compare/v0.5.0...v0.6.0) (2026-06-11)


### Features

* Charm-based animated, agent-safe CLI presentation layer ([0be41a6](https://github.com/jjfantini/pindex/commit/0be41a6b0400d03b04df6fc9462c5ba08528d1bc))
* **cli:** wire animated, agent-safe Charm UI into every command ([fa1ecc9](https://github.com/jjfantini/pindex/commit/fa1ecc9f0d6110b82b481fab20ddef478008e8e0))
* **index:** report build-stage progress from the Builder ([3eecf6d](https://github.com/jjfantini/pindex/commit/3eecf6de10cd9b6544ab00bc00e9d8133f9fe6cc))
* **llm:** emit resilience and cache events through an Observer hook ([1ab7d4b](https://github.com/jjfantini/pindex/commit/1ab7d4b2f734c0154d76e03a896b4d5ce439bb17))
* **ui:** add Charm-based terminal presentation layer ([dedd67d](https://github.com/jjfantini/pindex/commit/dedd67dafe157e6eddabe5f04206af2d96ac911d))

## [0.5.0](https://github.com/jjfantini/pindex/compare/v0.4.0...v0.5.0) (2026-06-11)


### Features

* **eval:** accumulating benchmark tree + one-command per-doc runs ([a56a449](https://github.com/jjfantini/pindex/commit/a56a4491650e26bfde4e7d865de368b355e6c2e8))
* **eval:** accumulating benchmark tree + one-command per-doc runs ([2754742](https://github.com/jjfantini/pindex/commit/2754742fb18b82156385bf33a805929498a5dba4))
* **eval:** accumulating benchmark tree with one-command per-doc runs ([62363b3](https://github.com/jjfantini/pindex/commit/62363b3fae7ec6493c2211cc47bcddd09328f5c9))

## [0.4.0](https://github.com/jjfantini/pindex/compare/v0.3.0...v0.4.0) (2026-06-10)


### Features

* **eval:** always save results, defaulting to .pindex/evals next to the workspace ([3a332ce](https://github.com/jjfantini/pindex/commit/3a332ce921d2105518ddc562a52ace135b3bb43d))
* **eval:** always save results, defaulting to .pindex/evals next to the workspace ([eb175ef](https://github.com/jjfantini/pindex/commit/eb175efbdc79adb4a8f82c3bd284ad52c310682e))
* **eval:** default eval output to .pindex/evals (always-saved, suffixed re-runs) ([348c539](https://github.com/jjfantini/pindex/commit/348c539b6490c1241d236d146c0f04fdc6bc82b7))

## [0.3.0](https://github.com/jjfantini/pindex/compare/v0.2.0...v0.3.0) (2026-06-10)


### Features

* **ask:** agentic loop at effort=high, answer verification at ultra ([748578b](https://github.com/jjfantini/pindex/commit/748578b7e586320a8585ad972d4d561490b2ea09))
* **ask:** agentic tree-search loop at effort=high, answer verification at ultra ([9f4e055](https://github.com/jjfantini/pindex/commit/9f4e055ec1903d59ccf6d154029c30f7e75bc379))
* effort-level ladder — agentic ask loop (high) + answer verification (ultra) ([b0d7310](https://github.com/jjfantini/pindex/commit/b0d73107b4dcd698089f5835c8d335966408801a))

## [0.2.0](https://github.com/jjfantini/pindex/compare/v0.1.1...v0.2.0) (2026-06-10)


### Features

* **cli:** add hidden docs command generating CLI reference markdown ([79a0433](https://github.com/jjfantini/pindex/commit/79a043301a06debb420ca45807608c9516c05961))

## [0.1.1](https://github.com/jjfantini/pindex/compare/v0.1.0...v0.1.1) (2026-06-09)


### Bug Fixes

* **ci:** goreleaser dirty-tree in release publish job ([b5fb931](https://github.com/jjfantini/pindex/commit/b5fb931e96d2c2e89e53ebfcceb83ed95384019c))
* **ci:** run goreleaser before downloading cgo artifacts (dirty tree) ([4974367](https://github.com/jjfantini/pindex/commit/4974367a9bcbf0553a45603754a3a29b56e3c206))

## 0.1.0 (2026-06-09)


### Features

* **license:** dual-license — Apache-2.0 first-party, AGPL for the MuPDF build ([24e52a5](https://github.com/jjfantini/pindex/commit/24e52a599e0b88ac86525f3811b95f2a22652e48))
* **llm:** split prompts into cacheable system + user, enable Anthropic prompt caching ([#20](https://github.com/jjfantini/pindex/issues/20)) ([9d69e75](https://github.com/jjfantini/pindex/commit/9d69e7593927b16671a59bb0cab0e70d6c62b237))


### Bug Fixes

* **release:** feat bumps minor pre-1.0 so first release is v0.1.0 ([152abe8](https://github.com/jjfantini/pindex/commit/152abe840a3ef7aab03f37ea5dc0a9040b589324))


### Miscellaneous Chores

* release as v0.1.0 ([4e16dd8](https://github.com/jjfantini/pindex/commit/4e16dd82ee4adbfba451c17bd9cd6862c34eedcf))
