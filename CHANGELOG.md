### v0.6.1

Minor release of the spec; the most important change is adjusting the type for
`annotations` to ACIdentifier (#442). This restores the ability for annotation
fields to be namespaced with DNS names.

Other changes:
- Added new maintainer (Ken Robertson)
- Created CHANGELOG.md to track changes instead of using git tags
- Fixed build scripts for FreeBSD (#433)
- Fixed acirenderer to work properly with empty images with just a rootfs
  directory (#428)
- Added `arm6vl` as valid arch for Linux (#440)

### v0.6.0

This is an important milestone release of the spec. Critically, there are two
backwards-incompatible schema changes from previous releases:
- Dependency references' `app` field has been renamed to the more accurate and
  unambiguous `imageName`: #397
- `ACName` has been redefined (with a stricter specification) to be suitable
  for relative names within a pod, and a new type, `ACIdentifier`, has been
  introduced for image, label and isolator names: #398

This release also sees the sections of the specification - image format,
discovery process, pods and executor definitions - being split into distinct
files. This is a first step towards clarifying the componentised nature of the
spec and the value in implementing individual sections.

Other changes of note in this release:
- Dependency references gained an optional `size` field. If this field is
  specified, executors should validate that the size of resolved dependency
  images is correct before attempting to retrieve them: #422
- The RFC3339 timestamp type definition was tweaked to clarify that it must
  include a T rather than a space: #410
- The spec now prescribes that ACEs must set the `container` environment
  variable to some value to indicate to applications that they are being run
  inside a container: #302
- Added support for 64-bit big-endian ARM architectures: #414
- Clarifications to the ports definition in the schema: #405
- Fixed a bug in the discovery code where it was mutating supplied objects:
  #412

### v0.5.2

This release features a considerable number of changes over the previous
(0.5.1) release. However, the vast majority are syntactical improvements to
clarity and wording in the text of the specification and do not change the
semantic behaviour in any significant way; hence, this should remain a
backwards-compatible release. As well as the changes to the spec itself, there
are various improvements to the schema/tooling code, including new
functionality in `actool`.

Some of the more notable changes since v0.5.1:
- `linux/aarch64`, `linux/armv7l` and `linux/armv7b` added as recognised
  os/arch combinations for images
- added contribution/governance policy and new maintainers
- added `cat-manifest` and `patch-manifest` subcommands to actool to manipulate
  existing ACIs
- added guidance around using authorization token (supplied in AC_METADATA_URL)
  for identifying pods to the metadata service
- reduced the set of required environment variables that executors must provide
- fixed consistency between schema code and spec for capabilities
- all TODOs removed from spec text and moved to GitHub issues
- several optimizations and fixes in acirenderer package

### v0.5.1

This is primarily a bugfix release to catch 2a342dac which resolves an issue
preventing PodManifests from being successfully serialized.

Other changes:
- Update validator to latest Pod spec changes
- Added /dev/pts and /dev/ptmx to Linux requirements
- Added a script to bump release versions
- Moved to using types.ACName in discovery code instead of strings

### v0.5.0

The major change in this release is the introduction of _pods_,
via #207 and #248. Pods are a refinement (and replacement) of the
previous ContainerRuntimeManifest concept that define the minimum
deployable, executable unit in the spec, as a grouping of one or
more applications. The move to pods implicitly resolves various
issues with the CRM in previous versions of the spec (e.g #83, #84)

Other fixes and changes in this release:
- fix static builds of the tooling on go1.4
- add ability to use proxy from environment for discovery
- fix inheritance of readOnly flag
- properly validate layouts with relative paths
- properly tar named pipes and ignore sockets
- add /dev/shm to required Linux environment

### v0.4.1

This is a minor bugfix release to fix marshalling of isolators.

### v0.4.0

Major changes and additions since v0.3.0:
- Reworked isolators to objects instead of strings and clarify limits vs
  reservations for resource isolators
- Introduced OS-specific requirements (e.g. device files on Linux)
- Moved much of the wording in the spec towards RFC2119 wording ("MUST", "MAY",
  "SHOULD", etc) to be more explicit about which parts are
  required/optional/recommended
- Greater explicitness around signing and encryption requirements
- Moved towards `.asc` filename extension for signatures
- Added MAINTAINERS
- Added an implementation guide
- Tighter restrictions on layout of ACI tars
- Numerous enhancements to discovery code
- Improved test coverage for various schema types

### v0.3.0

### v0.2.0

### v0.1.1

This marks the first versioned release of the app container spec.
