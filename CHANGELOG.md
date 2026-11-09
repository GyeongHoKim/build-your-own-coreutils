# Changelog

## [1.1.0](https://github.com/GyeongHoKim/build-your-own-coreutils/compare/v1.0.0...v1.1.0) (2026-11-09)


### Features

* **grade:** grade rm implementations ([c3a25e4](https://github.com/GyeongHoKim/build-your-own-coreutils/commit/c3a25e427cb7e6e29ad3f472ef78dfe51f6a1910))
* **rm:** add the rm command ([5551b85](https://github.com/GyeongHoKim/build-your-own-coreutils/commit/5551b85ecdbd5d0dca099dd98bca1e85650dc32a))
* **rm:** remove directory hierarchies ([066d6b1](https://github.com/GyeongHoKim/build-your-own-coreutils/commit/066d6b1f753fbb918b07162f567a51db511ae2cd))
* **rm:** remove siblings in parallel ([43afe13](https://github.com/GyeongHoKim/build-your-own-coreutils/commit/43afe1370467a426af70626e941eaff80d8a9f8e))
* **rm:** remove single entries per POSIX ([0558a94](https://github.com/GyeongHoKim/build-your-own-coreutils/commit/0558a94b621fb21745292266fe23ceabaf49885c))


### Bug Fixes

* **grade:** tell missing rm paths apart from stat errors ([6927937](https://github.com/GyeongHoKim/build-your-own-coreutils/commit/6927937cf99b839553314b936f5d234fd8fa02af))
* **rm:** check the opened directory for the root under "link/" ([e3c5455](https://github.com/GyeongHoKim/build-your-own-coreutils/commit/e3c54554aa88af3762ef208ead88691ff13709cc))

## 1.0.0 (2026-11-08)


### Features

* **filetime:** read access and modification times losslessly ([756e117](https://github.com/GyeongHoKim/build-your-own-coreutils/commit/756e1175c83a8e8896c21098ebc4cb1cce3bb5f9))
* **grade:** add timestamp fixtures for grading touch ([b8d4817](https://github.com/GyeongHoKim/build-your-own-coreutils/commit/b8d481715580d693600850dd84842535c95b1ded))
* **grade:** grade learner touch executables ([6d122ef](https://github.com/GyeongHoKim/build-your-own-coreutils/commit/6d122eff45b1c047feb8711f9de22cd2fac31e2a))
* touch ([0a22b7d](https://github.com/GyeongHoKim/build-your-own-coreutils/commit/0a22b7d954194dc8ee620098d6292b9d20474214))
* **touch:** add the touch command ([dc67eb4](https://github.com/GyeongHoKim/build-your-own-coreutils/commit/dc67eb4553b14ed01eb4b94f82038b5df29be503))
* **touch:** create files and update selected timestamps ([17e13c7](https://github.com/GyeongHoKim/build-your-own-coreutils/commit/17e13c7707c185ae5700d35dd9e957ee031bee4f))
* **touch:** parse -t timestamps strictly ([3772b94](https://github.com/GyeongHoKim/build-your-own-coreutils/commit/3772b940700eaa10159d030ca6deeabb7105c38b))
* **tree:** make my own tree ([9d11d1f](https://github.com/GyeongHoKim/build-your-own-coreutils/commit/9d11d1f3838117c0be8fac1d04b462ad2339455f))


### Bug Fixes

* **touch:** keep verbatim paths uncleaned when checking parents ([de74d7d](https://github.com/GyeongHoKim/build-your-own-coreutils/commit/de74d7def9855642e0116a4d532ceb9fd1619b8e))
* **tree:** use locale-aware entry sorting ([5380033](https://github.com/GyeongHoKim/build-your-own-coreutils/commit/53800334705c28d5d664fbef4a8975752a3307dd))
