# Changelog

## [0.9.0](https://github.com/devr-tools/szr/compare/v0.8.0...v0.9.0) (2026-07-02)


### Features

* **filters:** add declarative dedup/fold primitives and restore CompactLines duplicate folding ([bafe5a9](https://github.com/devr-tools/szr/commit/bafe5a952786e21ae143b71b619d22a0936a7c76))
* **filters:** collapse tree chains, keep representative grep matches, widen noise suppression ([fa87868](https://github.com/devr-tools/szr/commit/fa878680a764279d0ef1677574c64b3e4f79e9d9))
* **filters:** fold no-signal logs and unify diagnostic anchors in failure output ([6818b9b](https://github.com/devr-tools/szr/commit/6818b9bb6d4fec824c1d21be880790914640b0f8))
* **profiles:** compress git operation and go module download output ([dc36817](https://github.com/devr-tools/szr/commit/dc36817e659e53364acc51211362fb4e6351b280))
* **profiles:** route golangci-lint and staticcheck through the failure reducer ([1848079](https://github.com/devr-tools/szr/commit/1848079ce7a229c3f972114ec51003e351deae41))
* **profiles:** summarize kubectl mutations, docker transfers, and terraform plans ([53ae21c](https://github.com/devr-tools/szr/commit/53ae21c9141b7fde2f9807cdd1a9356206b6e96d))
* raise token savings with declarative folding, broader profile coverage, and engine tuning ([9fc299e](https://github.com/devr-tools/szr/commit/9fc299e90721f892a54250e4c6481a6dfa03c672))


### Bug Fixes

* **engine:** apply the never-worse-than-raw guard to all profiles ([f157097](https://github.com/devr-tools/szr/commit/f157097b6a46750e2f68a8f0fc08385614793025))
* **engine:** treat grep/ripgrep no-match exits as benign ([21f0c80](https://github.com/devr-tools/szr/commit/21f0c808df6a4ed66824d400037212531d06665f))
* **filters:** cap JSON structure rendering and strip OSC escape sequences ([46a8a0f](https://github.com/devr-tools/szr/commit/46a8a0f925ee45e0238caf47a8dfc9db72e49a00))


### Performance Improvements

* **engine:** bound history growth, enable adaptive budgets, prefer cheaper bypass summaries ([ea22557](https://github.com/devr-tools/szr/commit/ea225573deee3cc72cfeb302dd728aa6ad234b0c))

## [0.8.0](https://github.com/devr-tools/szr/compare/v0.7.0...v0.8.0) (2026-06-24)


### Features

* add "What's New" banner to menu header ([f5708e4](https://github.com/devr-tools/szr/commit/f5708e4ccf1f0817ed238d5f37432323a764a27c))
* add "What's New" banner to menu header ([fbd56cc](https://github.com/devr-tools/szr/commit/fbd56ccf78ffc3c63ff2ecaf123bdb3b1cbfb080))
* add new feature banner ([c803aba](https://github.com/devr-tools/szr/commit/c803abaa100dd75681ed995dc8c055293c91a10e))

## [0.7.0](https://github.com/devr-tools/szr/compare/v0.6.0...v0.7.0) (2026-06-22)


### Features

* add token-optimized rendering, git success-path summaries ([87d1242](https://github.com/devr-tools/szr/commit/87d124245d297aa00837e36b396a5a757c6571df))
* add token-optimized rendering, git success-path summaries, and discovery coverage improvements ([9836e03](https://github.com/devr-tools/szr/commit/9836e0379dd31c9fb89c079f7ee089f18454ce5f))
* refactor: resolve codeguard quality warnings in git and engine tests ([1aa7877](https://github.com/devr-tools/szr/commit/1aa787738a7a8457d451e426a693eb53f4c6f35f))
* refactor: split quality hotspots and restore CI coverage ([70f3e26](https://github.com/devr-tools/szr/commit/70f3e2663e9e2b99fc37bd4359ca48ba0d4eee92))

## [0.6.0](https://github.com/devr-tools/szr/compare/v0.5.0...v0.6.0) (2026-05-27)


### Features

* add clear command and harden git profile ([b01128b](https://github.com/devr-tools/szr/commit/b01128b6557ec131ae466b360fe10bc37ec9c4a3))
* add clear command and harden git profile ([2a57100](https://github.com/devr-tools/szr/commit/2a57100ad3919d9b68017fc3a64b4f768228c29a))

## [0.5.0](https://github.com/devr-tools/szr/compare/v0.4.0...v0.5.0) (2026-05-26)


### Features

* fix tests and updated the profiles strategies ([d6d4142](https://github.com/devr-tools/szr/commit/d6d4142eda1eb38733bba16d531e620945d9ec8d))
* update commands and profile strategy ([14913ef](https://github.com/devr-tools/szr/commit/14913efa02775253ad13283b52fed133160e856f))

## [0.4.0](https://github.com/devr-tools/szr/compare/v0.3.0...v0.4.0) (2026-05-26)


### Features

* add commands ([e23c282](https://github.com/devr-tools/szr/commit/e23c282c25b492cb6de819166a7d9392c2ec1d49))
* fix for pull request finding 'Useless assignment to local variable' ([66fb27a](https://github.com/devr-tools/szr/commit/66fb27a509e4262b931f99e437435e4fb02e64ad))
* fix test ([2ba144a](https://github.com/devr-tools/szr/commit/2ba144a76d78cfbcccaffdc8e89b2a5286016d91))

## [0.3.0](https://github.com/devr-tools/szr/compare/v0.2.0...v0.3.0) (2026-05-26)


### Features

* add commands add settings ([1caaf56](https://github.com/devr-tools/szr/commit/1caaf5699c2503bc409a748e238691ac8bf02406))
* add settings add commands ([e135d70](https://github.com/devr-tools/szr/commit/e135d704a21b5617403e635396355be1d9c7b4a0))
* add settings add commands ([bd47581](https://github.com/devr-tools/szr/commit/bd4758148a3cb3ee4dac4d5e358234366c2f25fb))
* fix commmits ([dc57b75](https://github.com/devr-tools/szr/commit/dc57b75c3ac3baa36780a0dfb7a0809b2a38b906))
* fix semgrep ([c0f6ce7](https://github.com/devr-tools/szr/commit/c0f6ce7d257ff4ca3df81b426309eb560187f518))
* release file ([d0c4b25](https://github.com/devr-tools/szr/commit/d0c4b25d82bcdeab84594801e79f9c66435172a1))


### Bug Fixes

* use semver-compatible release tags ([3ab318e](https://github.com/devr-tools/szr/commit/3ab318e069f6f7b55882880444aaac8a1d15326b))

## [0.2.0](https://github.com/devr-tools/szr/compare/szr-v0.1.0...szr-v0.2.0) (2026-05-25)


### Features

* fix-installed-aps ([87b56d9](https://github.com/devr-tools/szr/commit/87b56d981b701ae1411ebee596a0b2c116a04e3f))


### Bug Fixes

* save ([d510d6d](https://github.com/devr-tools/szr/commit/d510d6de09e9676ffa8cb4eb5086cf117f49dfcc))
* save ([99536fe](https://github.com/devr-tools/szr/commit/99536fe7b55e9adc9d3dec3654d1afcd82809b12))

## Changelog

All notable changes to `szr` will be tracked in this file.

This changelog is managed by `release-please` once the CI/CD workflows are enabled.
