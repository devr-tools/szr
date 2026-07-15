# Changelog

## [0.18.0](https://github.com/devr-tools/szr/compare/v0.17.0...v0.18.0) (2026-07-15)


### Features

* land user-defined filters, transcript insight commands, and coverage batch on main ([b4669f3](https://github.com/devr-tools/szr/commit/b4669f3d10c2d4c7d819b77e693038a3c6fb7097))
* output-guard invariant, tee retention, transparent prefixes, and new profiles ([82cefd8](https://github.com/devr-tools/szr/commit/82cefd89a041d2026bf94a8296911d38980ceb09))
* output-guard invariant, tee retention, transparent prefixes, and new profiles ([00faa8f](https://github.com/devr-tools/szr/commit/00faa8fc2c31356060833d30bf33201ba9cd765a))
* user-defined filters, transcript insight commands, and coverage batch ([2666200](https://github.com/devr-tools/szr/commit/2666200be3e084d42c44100c7f58af8cf7fdb210))
* user-defined filters, transcript insight commands, and coverage batch ([f2553d2](https://github.com/devr-tools/szr/commit/f2553d2b8b2596b5a6580bcc924d968456c8b54e))

## [0.17.0](https://github.com/devr-tools/szr/compare/v0.16.0...v0.17.0) (2026-07-03)


### Features

* **packaging:** distribute szr via npm and PyPI ([23157e0](https://github.com/devr-tools/szr/commit/23157e0a0e9d6b211b03af1331dfeb530d29b5ac))
* **packaging:** distribute szr via npm and PyPI ([942771c](https://github.com/devr-tools/szr/commit/942771ce53fc88fc502d42f315636d3b850586b4))


### Bug Fixes

* **cd:** grant id-token to the release.yml caller ([60be976](https://github.com/devr-tools/szr/commit/60be976b910b52869ab24738fc3793b38379f97b))
* **cd:** grant id-token to the release.yml caller ([a8439fa](https://github.com/devr-tools/szr/commit/a8439fa7074ec863bb55665fb8659dddd81d7859))

## [0.16.0](https://github.com/devr-tools/szr/compare/v0.15.0...v0.16.0) (2026-07-03)


### Features

* **engine:** route shell-wrapped and inline-eval commands to real profiles ([f30b23a](https://github.com/devr-tools/szr/commit/f30b23a2b602f2b23117487c639dac59c921a2fa))
* route shell-wrapped and node inline-eval commands to real profiles ([86ef530](https://github.com/devr-tools/szr/commit/86ef530169038414e9cd765dd86326ef61aee079))

## [0.15.0](https://github.com/devr-tools/szr/compare/v0.14.0...v0.15.0) (2026-07-03)


### Features

* **cli:** stdin pipe mode with content-sniffed summarization ([3b170da](https://github.com/devr-tools/szr/commit/3b170dac27881b7d193969c292ae345babf27cef))
* delta rendering, swarm context scopes, and stdin pipe mode ([2e5fa2c](https://github.com/devr-tools/szr/commit/2e5fa2cf1ae66e3ce8bb58d2bcc81032dca94260))
* **engine:** delta rendering for changed re-runs and swarm context scopes ([6c1ecd5](https://github.com/devr-tools/szr/commit/6c1ecd521b09bdbda93eaa1166f7e4e8a317567c))


### Bug Fixes

* **engine:** enforce never-worse-than-raw on the final display ([6c7265a](https://github.com/devr-tools/szr/commit/6c7265ac2d795c421995fad506172c41171ec03b))

## [0.14.0](https://github.com/devr-tools/szr/compare/v0.13.0...v0.14.0) (2026-07-03)


### Features

* **engine:** session dedup with reference expansion ([4c375cf](https://github.com/devr-tools/szr/commit/4c375cf4d4a97c84dd7616c6a2205a7c3600b3c7))
* **filters:** tabular rendering for uniform JSON arrays ([2dcf4f8](https://github.com/devr-tools/szr/commit/2dcf4f83846fd8409ba2f2f275a46e76e90522f0))
* **filters:** tier-priority self-capping and binary-aware fallback rendering ([6e5fe24](https://github.com/devr-tools/szr/commit/6e5fe242ed65755dad1f9a05fdb177972d2b55e4))
* session dedup with reference expansion, tabular JSON, and fidelity depth ([5213cc2](https://github.com/devr-tools/szr/commit/5213cc2661718e819037b2c6ded4e84fcc83d3f5))


### Bug Fixes

* **engine:** honest small-output summaries and hotspot selection ([1d64b09](https://github.com/devr-tools/szr/commit/1d64b091b10b8d0505ae0b260986abeb8283e91c))

## [0.13.0](https://github.com/devr-tools/szr/compare/v0.12.1...v0.13.0) (2026-07-03)


### Features

* **engine:** retention verifier repairs renders that drop critical facts ([7eb537c](https://github.com/devr-tools/szr/commit/7eb537caafbfb2f882f2b8ccf4d48c3e9b4b53e2))
* **filters:** anomaly-aware lists, full diff inventories, and deeper build detail ([00d35fa](https://github.com/devr-tools/szr/commit/00d35faf66f3e8fb0f5453127c08da6634ee111e))
* **filters:** keep failing-test identifiers and error codes in runner renders ([ec31750](https://github.com/devr-tools/szr/commit/ec317507ae1984513fb95d3aeb701779e6deca15))
* retention verifier and fidelity-depth improvements across renders ([7b4d106](https://github.com/devr-tools/szr/commit/7b4d1067142b164de2e8461816fad463084608cb))


### Bug Fixes

* **cli:** spread surfaces genuinely poor savings and skips always-failing loosen advice ([522f315](https://github.com/devr-tools/szr/commit/522f315c0fabab84ed0060f77cc36b0203d3a3eb))

## [0.12.1](https://github.com/devr-tools/szr/compare/v0.12.0...v0.12.1) (2026-07-03)


### Bug Fixes

* **engine:** post-render guards must stand down when capture is truncated ([fdc6af8](https://github.com/devr-tools/szr/commit/fdc6af84f534670c0791876f2e9bc59d85ef410e))
* post-render guards must not compare against truncated capture ([575e7c8](https://github.com/devr-tools/szr/commit/575e7c8f8c5a6856f6160e9688199d2c14a53a75))

## [0.12.0](https://github.com/devr-tools/szr/compare/v0.11.0...v0.12.0) (2026-07-03)


### Features

* **filters:** severity-aware summaries for log files ([742b4ab](https://github.com/devr-tools/szr/commit/742b4ab5d7314f68661279f2c9f32adfc713f7bb))
* **profiles:** summarize raw gh api JSON responses ([eee2d1f](https://github.com/devr-tools/szr/commit/eee2d1f4a54fd6dfd02ec43c9600d7be9b502072))
* summarize raw gh api JSON responses ([efbdbf1](https://github.com/devr-tools/szr/commit/efbdbf1545a6a30b100771f848e6deb89add44b4))


### Bug Fixes

* **cli:** delegate unsupported builtin argv to native binaries ([0b207f5](https://github.com/devr-tools/szr/commit/0b207f58a54892cabc4b8125daa7cf0c2e977a46))
* **cli:** report spread savings over filtered traffic and skip zero-output hotspots ([b13d9f3](https://github.com/devr-tools/szr/commit/b13d9f35f754d5cf612bd71beae826389736b22b))
* **cli:** restore exported localcmd signatures for API compatibility ([bd36457](https://github.com/devr-tools/szr/commit/bd36457f09394d0cbeb4ce08dbd2ce731102ffdb))
* **engine:** fidelity floor for the compression contract ([8e80a3b](https://github.com/devr-tools/szr/commit/8e80a3b683baf7a367dd2fb2de7df01c0d5b852a))
* **engine:** retry command start on ETXTBSY ([98eabf7](https://github.com/devr-tools/szr/commit/98eabf75b7cc2ea3d66037f95069261ceac7c70d))
* fidelity-first filtering — never destroy the answer ([db8d6bd](https://github.com/devr-tools/szr/commit/db8d6bdc325a509fbcacf91d20a471b9517c011d))
* **filters:** list small find results verbatim and lead gh pr view with its headline ([f8033f2](https://github.com/devr-tools/szr/commit/f8033f28a75a5ef4fbee30369204a6270a8ad969))
* **profiles:** git rewrites respect explicit user flags and small diffs keep content ([bffdd3f](https://github.com/devr-tools/szr/commit/bffdd3fc442f4d2cac3b61da5b6069138fca8260))

## [0.11.0](https://github.com/devr-tools/szr/compare/v0.10.0...v0.11.0) (2026-07-02)


### Features

* **history:** tag proxied runs and exclude them from savings analysis ([cc0870c](https://github.com/devr-tools/szr/commit/cc0870c47cec3abb164fdc0368cf0f6ddb7b22f3))
* **profiles:** summarize gh pr checks tables and fold watch repaints ([ae3d240](https://github.com/devr-tools/szr/commit/ae3d24062970759f7907336551cbfb3bec3f931b))


### Bug Fixes

* correct compression-contract budgeting and clean up spread analytics ([491ba85](https://github.com/devr-tools/szr/commit/491ba8509622b66df3322ade7d2b53f49e570819))
* **engine:** budget the compression contract against true raw token count ([6451378](https://github.com/devr-tools/szr/commit/6451378c8838652d316cdf867922a74a2e3d7fd1))
* **profiles:** route pattern-only grep through the grep profile ([107553a](https://github.com/devr-tools/szr/commit/107553ae51c30092076d78fc88d3c287c7a03d06))

## [0.10.0](https://github.com/devr-tools/szr/compare/v0.9.0...v0.10.0) (2026-07-02)


### Features

* **cli:** drive What's New banner from embedded changelog and show version on spread ([a9e1be7](https://github.com/devr-tools/szr/commit/a9e1be7c0f03958adc0d42ba239fe24c40dc31a4))
* **cli:** drive What's New banner from embedded changelog and show version on spread ([6a39817](https://github.com/devr-tools/szr/commit/6a398171ff4b151034a7ff5e9e4062b885927dbe))

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
