# App Container 

## Overview

This repository contains schema definitions and tools for the App Container specification.
See [SPEC.md](SPEC.md) for details of the specification itself.

_Thank you to Tobi Knaup and Ben Hindman from Mesosphere, and the Pivotal Engineering team for providing initial feedback on the spec._

- `schema` contains JSON definitions of the different constituent formats of the spec (the _Image Manifest_ and the _Container Runtime Manifest_). These JSON schemas also handle validation of the manifests through their Marshal/Unmarshal implementations.
  - `schema/types` contains various types used by the Manifest types to enforce validation
- `ace` contains a tool intended to be run within an _Application Container Executor_ to validate that the ACE has set up the container environment correctly. This tool can be built into an ACI image ready for running on an executor by using the `build_aci` script.
- `actool` contains a tool for building and validating images and manifests that meet the App Container specifications.

## Building ACIs 

`actool` can be used to build an Application Container Image from an [Image Layout](SPEC.md#image-layout) - that is, from an Image Manifest and an application root filesystem (rootfs). 

For example, to build a simple ACI (in this case consisting of a single binary), one could do the following:
```
$ find /tmp/my-app/
/tmp/my-app/
/tmp/my-app/manifest
/tmp/my-app/rootfs
/tmp/my-app/rootfs/bin
/tmp/my-app/rootfs/bin/my-app
$ cat /tmp/my-app/manifest
{
    "acKind": "ImageManifest",
    "acVersion": "0.1.1",
    "name": "my-app",
    "labels": [
        {"name": "os", "val": "linux"},
        {"name": "arch", "val": "amd64"}
    ],
    "app": {
        "exec": [
            "/bin/my-app"
        ],
        "user": "0",
        "group": "0"
    }
}
$ actool build /tmp/my-app/ /tmp/my-app.aci
```

Since an ACI is simply an (optionally compressed) tar file, we can inspect the created file with simple tools:

```
$ tar tvf /tmp/my-app.aci
drwxrwxr-x 1000/1000         0 2014-12-10 10:33 rootfs
drwxrwxr-x 1000/1000         0 2014-12-10 10:36 rootfs/bin
-rwxrwxr-x 1000/1000   5988728 2014-12-10 10:34 rootfs/bin/my-app
-rw-r--r-- root/root       332 2014-12-10 20:40 manifest
```

and verify that the manifest was embedded appropriately
```
tar xf /tmp/my-app.aci manifest -O | python -m json.tool
{
    "acKind": "ImageManifest",
    "acVersion": "0.1.1",
    "annotations": null,
    "app": {
        "environment": {},
        "eventHandlers": null,
        "exec": [
            "/bin/my-app"
        ],
        "group": "0",
        "isolators": null,
        "mountPoints": null,
        "ports": null,
        "user": "0"
    },
    "dependencies": null,
    "labels": [
        {
            "name": "os",
            "val": "linux"
        },
        {
            "name": "arch",
            "val": "amd64"
        }
    ],
    "name": "my-app",
    "pathWhitelist": null
}
```

## Validating App Container implementations

`actool validate` can be used by implementations of the App Container Specification to check that files they produce conform to the expectations.

### Validating Image Manifests and Container Runtime Manifests

To validate one of the two manifest types in the specification, simply run `actool validate` against the file.

```
$ actool validate ./image.json
$ echo $?
0
```

Multiple arguments are supported, and more output can be enabled with `-debug`:

```
$ actool -debug validate image1.json image2.json
image1.json: valid ImageManifest
image2.json: valid ImageManifest
```

`actool` will automatically determine which type of manifest it is checking (by using the `acKind` field common to all manifests), so there is no need to specify which type of manifest is being validated:
```
$ actool -debug validate /tmp/my_container
/tmp/my_container: valid ContainerRuntimeManifest
```

If a manifest fails validation the first error encountered is returned along with a non-zero exit status:
```
$ actool validate nover.json
nover.json: invalid ImageManifest: acVersion must be set
$ echo $?
1
```

### Validating ACIs and layouts

Validating ACIs or layouts is very similar to validating manifests: simply run the `actool validate` subcommmand directly against an image or directory, and it will determine the type automatically:
```
$ actool validate app.aci
$ echo $?
0
$ actool -debug validate app.aci
app.aci: valid app container image
```

```
$ actool -debug validate app_layout/
app_layout/: valid image layout
```

To override the type detection and force `actool validate` to validate as a particular type (image, layout or manifest), use the `--type` flag:

```
actool -debug validate -type appimage hello.aci
hello.aci: valid app container image
```

### Validating App Container Executors (ACEs)

The [`ace`](ace/) package contains a simple go application, the _ACE validator_, which can be used to validate app container executors by checking certain expectations about the environment in which it is run: for example, that the appropriate environment variables and mount points are set up as defined in the specification.

To use the ACE validator, first compile it into an ACI using the supplied `build_aci` script:
```
$ app-container/ace/build_aci 

You need a passphrase to unlock the secret key for
user: "Joe Bloggs (Example, Inc) <joe@example.com>"
4096-bit RSA key, ID E14237FD, created 2014-03-31

Wrote main layout to      bin/ace_main_layout
Wrote unsigned main ACI   bin/ace_validator_main.aci
Wrote main layout hash    bin/sha256-f7eb89d44f44d416f2872e43bc5a4c6c3e12c460e845753e0a7b28cdce0e89d2
Wrote main ACI signature  bin/ace_validator_main.sig

You need a passphrase to unlock the secret key for
user: "Joe Bloggs (Example, Inc) <joe@example.com>"
4096-bit RSA key, ID E14237FD, created 2014-03-31

Wrote sidekick layout to      bin/ace_sidekick_layout
Wrote unsigned sidekick ACI   bin/ace_validator_sidekick.aci
Wrote sidekick layout hash    bin/sha256-13b5598069dbf245391cc12a71e0dbe8f8cdba672072135ebc97948baacf30b2
Wrote sidekick ACI signature  bin/ace_validator_sidekick.sig

```

As can be seen, the script generates two ACIs: `ace_validator_main.aci`, the main entrypoint to the validator, and `ace_validator_sidekick.aci`, a sidekick application. The sidekick is used to validate that an ACE implementation properly handles running multiple applications in a container (for example, that they share a mount namespace), and hence both ACIs should be run together in a layout to validate proper ACE behaviour. The script also generates detached signatures which can be verified by the ACE.

When running the ACE validator, output is minimal if tests pass, and errors are reported as they occur - for example:

```
preStart OK
main OK
sidekick OK
postStop OK
```

or, on failure:
```
main FAIL
==> file "/prestart" does not exist as expected
==> unexpected environment variable "WINDOWID" set
==> timed out waiting for /db/sidekick
```
