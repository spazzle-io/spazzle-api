# Changelog

## [0.1.4](https://github.com/spazzle-io/spazzle-api/compare/v0.1.3...v0.1.4) (2026-06-08)


### Features

* add aggressive artist liveness check ([#106](https://github.com/spazzle-io/spazzle-api/issues/106)) ([ac9ef58](https://github.com/spazzle-io/spazzle-api/commit/ac9ef5862609252a6198989c452d18e55a03acd5))
* add AGPL license ([#64](https://github.com/spazzle-io/spazzle-api/issues/64)) ([ed72cab](https://github.com/spazzle-io/spazzle-api/commit/ed72cabdb17b8cc701d72f856588c8e7fedaf346))
* add server treasuries ([#128](https://github.com/spazzle-io/spazzle-api/issues/128)) ([33bfe09](https://github.com/spazzle-io/spazzle-api/commit/33bfe093f3f7d8839c098918c974f8d6ba407eb4))
* add temporal ([#93](https://github.com/spazzle-io/spazzle-api/issues/93)) ([031d51f](https://github.com/spazzle-io/spazzle-api/commit/031d51fbd1ef8bc8c66491c8dcc9dde8825beba0))
* **common:** add s3 compatible storage shared lib ([#108](https://github.com/spazzle-io/spazzle-api/issues/108)) ([367cac5](https://github.com/spazzle-io/spazzle-api/commit/367cac52e266245adfb86bc291e4ac80b0593776))
* **gameplay:** add create server endpoint ([#52](https://github.com/spazzle-io/spazzle-api/issues/52)) ([40a29e1](https://github.com/spazzle-io/spazzle-api/commit/40a29e1cf1f8f654e52fea27da46d1cdb08bf50a))
* **gameplay:** add game service endpoints ([#111](https://github.com/spazzle-io/spazzle-api/issues/111)) ([d63b0b6](https://github.com/spazzle-io/spazzle-api/commit/d63b0b6209dfc3551ec9af58392cd52228690374))
* **gameplay:** add gameplay service ([#50](https://github.com/spazzle-io/spazzle-api/issues/50)) ([e919794](https://github.com/spazzle-io/spazzle-api/commit/e919794cd5adaf190f506f413f6426d30a77f90e))
* **gameplay:** add join server http endpoint ([#91](https://github.com/spazzle-io/spazzle-api/issues/91)) ([95a6eb1](https://github.com/spazzle-io/spazzle-api/commit/95a6eb1a7b333b60be1fd0c287d5fdc7cc063039))
* **gameplay:** add join server ws endpoint ([#83](https://github.com/spazzle-io/spazzle-api/issues/83)) ([3aea9b3](https://github.com/spazzle-io/spazzle-api/commit/3aea9b3ae1be22a3908076d06dec1f891bf1f646))
* **gameplay:** add server admin endpoints ([#65](https://github.com/spazzle-io/spazzle-api/issues/65)) ([5ff93bf](https://github.com/spazzle-io/spazzle-api/commit/5ff93bfdd55253c288e1a7bcd66ce67539cd6689))
* **gameplay:** add server fetch endpoints ([#54](https://github.com/spazzle-io/spazzle-api/issues/54)) ([4837d8f](https://github.com/spazzle-io/spazzle-api/commit/4837d8ff48d1639bdff0f22e9e4675cf297dda2d))
* **gameplay:** add server mutation endpoints ([#58](https://github.com/spazzle-io/spazzle-api/issues/58)) ([b485e2b](https://github.com/spazzle-io/spazzle-api/commit/b485e2b821624838540953e77a305f1974198370))
* **gameplay:** add word service endpoints ([#76](https://github.com/spazzle-io/spazzle-api/issues/76)) ([3218d41](https://github.com/spazzle-io/spazzle-api/commit/3218d41cf5c36bb57aeb8e1d836595b3f0d87ce0))
* **gameplay:** implement game streams cleanup and persistence ([#109](https://github.com/spazzle-io/spazzle-api/issues/109)) ([db7f4c5](https://github.com/spazzle-io/spazzle-api/commit/db7f4c53f3a2005f068c3c9a85da78d47a9b03ce))
* **gameplay:** implement struct marker type ([#144](https://github.com/spazzle-io/spazzle-api/issues/144)) ([e187a8e](https://github.com/spazzle-io/spazzle-api/commit/e187a8e4fc6e453ab9a3d51ebe9d0489ed42f3e6))
* **gameplay:** implement temporal game server workflow  ([#96](https://github.com/spazzle-io/spazzle-api/issues/96)) ([558e4f8](https://github.com/spazzle-io/spazzle-api/commit/558e4f8de2037a4d48cf06694d1126af705e9188))
* **gameplay:** isolate gameserver package ([#92](https://github.com/spazzle-io/spazzle-api/issues/92)) ([87aa646](https://github.com/spazzle-io/spazzle-api/commit/87aa6466c20f144d07201646915f3b1244fc11f7))
* implement game replay endpoint ([#107](https://github.com/spazzle-io/spazzle-api/issues/107)) ([48da3a9](https://github.com/spazzle-io/spazzle-api/commit/48da3a9ab770ad59240396309510b5c199c6a29b))
* merge services to single proto file ([#63](https://github.com/spazzle-io/spazzle-api/issues/63)) ([2ce9e09](https://github.com/spazzle-io/spazzle-api/commit/2ce9e09dc7ffea0dd3a699ff2038816e7d66aa51))
* migrate reqs to google pb wrappers ([#60](https://github.com/spazzle-io/spazzle-api/issues/60)) ([eb6136d](https://github.com/spazzle-io/spazzle-api/commit/eb6136d1454649e307ec3608967e18c5e5c185b1))
* remove userId as a param to verify access token request ([#61](https://github.com/spazzle-io/spazzle-api/issues/61)) ([eac5904](https://github.com/spazzle-io/spazzle-api/commit/eac5904a02ea8119551ccb82bc3c8d4f639fb18e))
* **users:** migrate list users endpoint to use keyset pagination ([#59](https://github.com/spazzle-io/spazzle-api/issues/59)) ([0eb5d6d](https://github.com/spazzle-io/spazzle-api/commit/0eb5d6dbf83de87124c9300e6f63742a27e54c43))


### Bug Fixes

* **gameplay:** reset treasury deployer nonce on development env ([#131](https://github.com/spazzle-io/spazzle-api/issues/131)) ([3875e04](https://github.com/spazzle-io/spazzle-api/commit/3875e040d2f2388f84cc81894d88df38b4cb02d4))


### Performance Improvements

* **gameplay:** optimize round/game ended ws payloads ([#129](https://github.com/spazzle-io/spazzle-api/issues/129)) ([5ea407f](https://github.com/spazzle-io/spazzle-api/commit/5ea407fde19998a5fc99496180046e0a2b16d1fa))
* increment max stream len ([#141](https://github.com/spazzle-io/spazzle-api/issues/141)) ([d54437a](https://github.com/spazzle-io/spazzle-api/commit/d54437a9d91d59d2163a8b558f01820af03b1101))
* optimize redis streams event bus ([#140](https://github.com/spazzle-io/spazzle-api/issues/140)) ([9611a30](https://github.com/spazzle-io/spazzle-api/commit/9611a305ba44791f98049e7e0fd832a356b0f4c7))

## [0.1.3](https://github.com/spazzle-io/spazzle-api/compare/v0.1.2...v0.1.3) (2025-08-10)


### Features

* **auth:** report account already exists error on authenticate endpoint ([#29](https://github.com/spazzle-io/spazzle-api/issues/29)) ([81b62b4](https://github.com/spazzle-io/spazzle-api/commit/81b62b4e79f8653658dba89916aaff79c2264b5a))
* **auth:** select required columns sql ([#38](https://github.com/spazzle-io/spazzle-api/issues/38)) ([3f0070b](https://github.com/spazzle-io/spazzle-api/commit/3f0070b367375e2035818ee2b062a082309bc3a1))
* **users:** add async background task processor ([#31](https://github.com/spazzle-io/spazzle-api/issues/31)) ([46fdec3](https://github.com/spazzle-io/spazzle-api/commit/46fdec37d14a16d8b7aeba22d1a2610d0f1e9eea))
* **users:** add authenticate user rpc ([#41](https://github.com/spazzle-io/spazzle-api/issues/41)) ([81c9c08](https://github.com/spazzle-io/spazzle-api/commit/81c9c080342e0e39ead5681c0c9c3dce32ee9dfe))
* **users:** add create user db tx ([#28](https://github.com/spazzle-io/spazzle-api/issues/28)) ([b4be3ce](https://github.com/spazzle-io/spazzle-api/commit/b4be3ce2dcee8678eabc3a99d5347d5dc0a8267f))
* **users:** add create user rpc ([#40](https://github.com/spazzle-io/spazzle-api/issues/40)) ([63ec263](https://github.com/spazzle-io/spazzle-api/commit/63ec263a01c60da52acde53678792ea1a1577dd5))
* **users:** add ens fields to user table ([#30](https://github.com/spazzle-io/spazzle-api/issues/30)) ([d31b60b](https://github.com/spazzle-io/spazzle-api/commit/d31b60b64ea05c498d3cc890a497d585abbd550d))
* **users:** add get user endpoints ([#43](https://github.com/spazzle-io/spazzle-api/issues/43)) ([fecf81c](https://github.com/spazzle-io/spazzle-api/commit/fecf81cc1135ce6f1d5900f8dfd6000ba9f3c516))
* **users:** add list users endpoint ([#44](https://github.com/spazzle-io/spazzle-api/issues/44)) ([5045036](https://github.com/spazzle-io/spazzle-api/commit/50450369886a20bbbf5ee2ffb2bab1f1b381c3e0))
* **users:** add rate limits ([#46](https://github.com/spazzle-io/spazzle-api/issues/46)) ([dea2b11](https://github.com/spazzle-io/spazzle-api/commit/dea2b1161729871c5885ad4134fb138e178645fd))
* **users:** add update user endpoint ([#45](https://github.com/spazzle-io/spazzle-api/issues/45)) ([16efd0b](https://github.com/spazzle-io/spazzle-api/commit/16efd0b0b88d2a05a5b742277d6c3132a2c29543))
* **users:** add users service ([#26](https://github.com/spazzle-io/spazzle-api/issues/26)) ([75f532b](https://github.com/spazzle-io/spazzle-api/commit/75f532bf4f45d81bad286dbb92e589caea5da334))
* **users:** refactor authenticate endpoint to handle both new and existing users ([#42](https://github.com/spazzle-io/spazzle-api/issues/42)) ([78930d2](https://github.com/spazzle-io/spazzle-api/commit/78930d24499d294ea45f906d575451a7473e3864))
* **users:** rollback async background task processor ([#36](https://github.com/spazzle-io/spazzle-api/issues/36)) ([bc9f7fa](https://github.com/spazzle-io/spazzle-api/commit/bc9f7fab9672c464a1604b3f651b21b56632575a))
* **users:** select required columns sql ([#37](https://github.com/spazzle-io/spazzle-api/issues/37)) ([3eecf13](https://github.com/spazzle-io/spazzle-api/commit/3eecf139ab9772c20b4c2c97a21ba3e52a3d35e6))

## [0.1.2](https://github.com/spazzle-io/spazzle-api/compare/v0.1.1...v0.1.2) (2025-07-19)


### Features

* **auth:** add rate limits ([#24](https://github.com/spazzle-io/spazzle-api/issues/24)) ([8f6960b](https://github.com/spazzle-io/spazzle-api/commit/8f6960bfb0e8de443fd96a16ef676fea578b46a8))

## [0.1.1](https://github.com/spazzle-io/spazzle-api/compare/v0.1.0...v0.1.1) (2025-07-17)


### Features

* **auth:** add authenticate db tx ([#20](https://github.com/spazzle-io/spazzle-api/issues/20)) ([65248e5](https://github.com/spazzle-io/spazzle-api/commit/65248e5cb69f270ec64b8c75952ad1fd872c41f8))
* **auth:** add authenticate endpoint ([#16](https://github.com/spazzle-io/spazzle-api/issues/16)) ([c8ff4bc](https://github.com/spazzle-io/spazzle-api/commit/c8ff4bc0dbbbee29c76c2dd25d84c1b651bc8454))
* **auth:** add authz middleware ([#14](https://github.com/spazzle-io/spazzle-api/issues/14)) ([c192550](https://github.com/spazzle-io/spazzle-api/commit/c1925508deb53e8ee47d9eb0dd319412f2643fb0))
* **auth:** add get `SIWE` payload endpoint ([#15](https://github.com/spazzle-io/spazzle-api/issues/15)) ([fb55d2e](https://github.com/spazzle-io/spazzle-api/commit/fb55d2e7013b484a717c4c62aeeea5af5293d51e))
* **auth:** add refresh access token endpoint ([#22](https://github.com/spazzle-io/spazzle-api/issues/22)) ([5ea97ed](https://github.com/spazzle-io/spazzle-api/commit/5ea97edf63a7911264ca30b1049bf46ae28076c3))
* **auth:** add revoke refresh tokens endpoint ([#23](https://github.com/spazzle-io/spazzle-api/issues/23)) ([2f9aaeb](https://github.com/spazzle-io/spazzle-api/commit/2f9aaeb7c7a608283ea8de2f1d0593f213a36e2b))
* **auth:** add server middleware ([#12](https://github.com/spazzle-io/spazzle-api/issues/12)) ([c174fb2](https://github.com/spazzle-io/spazzle-api/commit/c174fb2e490b7a3da6c9574354913a377e52c7b5))
* **auth:** add verify access token endpoint ([#17](https://github.com/spazzle-io/spazzle-api/issues/17)) ([bc5bf51](https://github.com/spazzle-io/spazzle-api/commit/bc5bf515ea396f615af364f84c30723b7539b66b))
* **auth:** create database records ([#7](https://github.com/spazzle-io/spazzle-api/issues/7)) ([d7457e9](https://github.com/spazzle-io/spazzle-api/commit/d7457e91f89629c5b868293a8062dc3a534d202f))
* **auth:** setup auth server ([#10](https://github.com/spazzle-io/spazzle-api/issues/10)) ([fb7b1d8](https://github.com/spazzle-io/spazzle-api/commit/fb7b1d874cdc767ab6956dfe6e4deb6adb20a6f1))


### Bug Fixes

* **auth:** admin accounts user ID check ([#21](https://github.com/spazzle-io/spazzle-api/issues/21)) ([c4b836f](https://github.com/spazzle-io/spazzle-api/commit/c4b836f02f76233ade45e58de3028045030810f8))
* switch to googleapis/release-please-action ([#4](https://github.com/spazzle-io/spazzle-api/issues/4)) ([76330cc](https://github.com/spazzle-io/spazzle-api/commit/76330ccabeb2813c6865767eda38ddcb05d79614))
