# Changelog

### [1.6.1](https://www.github.com/devbranch-vadym/portainerssh/compare/v1.6.0...v1.6.1) (2022-10-28)


### Bug Fixes

* endpoint ID hardcoded in WS URL ([53dbfd5](https://www.github.com/devbranch-vadym/portainerssh/commit/53dbfd50f74ca21749609fbabc1761749034b522))

## [1.6.0](https://www.github.com/devbranch-vadym/portainerssh/compare/v1.5.0...v1.6.0) (2021-12-23)


### Features

* support returning container command exit code ([f981486](https://www.github.com/devbranch-vadym/portainerssh/commit/f981486a0763ee3fafae4785b748be50b3aed3f2))

## [1.5.0](https://www.github.com/devbranch-vadym/portainerssh/compare/v1.4.0...v1.5.0) (2021-09-28)


### Features

* support running in non-interactive shells ([24c601c](https://www.github.com/devbranch-vadym/portainerssh/commit/24c601c67943e8fa45cbee4cefed5907a019d926))

## [1.4.0](https://www.github.com/devbranch-vadym/portainerssh/compare/v1.3.0...v1.4.0) (2021-09-17)


### Features

* use shlex.Split to parse command as array ([f200878](https://www.github.com/devbranch-vadym/portainerssh/commit/f2008781e22e21bd1b399f0872f0960a85884f17))

## [1.3.0](https://www.github.com/devbranch-vadym/portainerssh/compare/v1.2.2...v1.3.0) (2021-08-01)


### Features

* implement realtime terminal resize handling ([21bc1f3](https://www.github.com/devbranch-vadym/portainerssh/commit/21bc1f32f69f50ec04ba72fd07129742e42f1149))

### [1.2.2](https://www.github.com/devbranch-vadym/portainerssh/compare/v1.2.1...v1.2.2) (2021-07-30)


### Bug Fixes

* handle unexpected EOF error when websocket is being closed by Portainer ([bd3a20c](https://www.github.com/devbranch-vadym/portainerssh/commit/bd3a20ca7ef740cbd9b97f5aa7c691aadc0da450))

### [1.2.1](https://www.github.com/devbranch-vadym/portainerssh/compare/v1.2.0...v1.2.1) (2021-07-30)


### Bug Fixes

* trigger release build ([548f170](https://www.github.com/devbranch-vadym/portainerssh/commit/548f170ca8293a712a1aee6ee6fc426c46752860))

## [1.2.0](https://www.github.com/devbranch-vadym/portainerssh/compare/v1.1.0...v1.2.0) (2021-07-29)


### Features

* implement running commands as a different user ([cb271eb](https://www.github.com/devbranch-vadym/portainerssh/commit/cb271ebd85f6a017f7f4cf033e753a033c7ff204))

## [1.1.0](https://www.github.com/devbranch-vadym/portainerssh/compare/v1.0.2...v1.1.0) (2021-07-29)


### Features

* implement matching container names with wildcards ([c759db0](https://www.github.com/devbranch-vadym/portainerssh/commit/c759db0ec3e70d98d18389ca4e381c1b6e85162f))
* initial support for terminal resizing ([e236a35](https://www.github.com/devbranch-vadym/portainerssh/commit/e236a35c623fb7d036ad8e60e26adbeb13f6e9d1))

### [1.0.2](https://www.github.com/devbranch-vadym/portainerssh/compare/v1.0.1...v1.0.2) (2021-07-29)


### Bug Fixes

* **ci:** fix release builds path ([9a4d99b](https://www.github.com/devbranch-vadym/portainerssh/commit/9a4d99bbc88b59d1b732448d3fd98865ffb045ab))

### [1.0.1](https://www.github.com/devbranch-vadym/portainerssh/compare/v1.0.0...v1.0.1) (2021-07-28)


### Bug Fixes

* force rebuild release ([e80d8b2](https://www.github.com/devbranch-vadym/portainerssh/commit/e80d8b2f4973dd5e8f8ded846731270d7492824c))

## [1.0.0](https://www.github.com/devbranch-vadym/portainerssh/compare/v0.0.2...v1.0.0) (2021-07-28)


### ⚠ BREAKING CHANGES

* Portainer has endpoints and it's a different thing

### Features

* add --command flag ([ffc5444](https://www.github.com/devbranch-vadym/portainerssh/commit/ffc5444b13d80f480d87acfbfa8c8b12aa2091e7))
* introduce Portainer endpoints support ([1252a38](https://www.github.com/devbranch-vadym/portainerssh/commit/1252a3810be7686ec75e23d38b8d020c658eb79b))


### Code Refactoring

* rename existing usages of 'endpoint' with 'api url' since ([6549f13](https://www.github.com/devbranch-vadym/portainerssh/commit/6549f13b22f036094849fc71c48d5b5bb962832f))
