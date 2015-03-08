# App Container Specification

_For version information, see [VERSION](VERSION)_

The "App Container" defines an image format, image discovery mechanism and execution environment that can exist in several independent implementations. The core goals include:

* Design for fast downloads and starts of the containers
* Ensure images are cryptographically verifiable and highly-cacheable
* Design for composability and independent implementations
* Use common technologies for crypto, archive, compression and transport
* Use the DNS namespace to name and discover container images

To achieve these goals this specification is split out into a number of smaller sections.

1. The **[App Container Image](#app-container-image)** defines: how files are assembled together into a single image, verified on download and placed onto disk to be run.

2. The **[App Container Executor](#app-container-executor)** defines: how an app container image on disk is run and the environment it is run inside including cgroups, namespaces and networking.

    * The [Metadata Server](#app-container-metadata-service) defines how a container can introspect and get a cryptographically verifiable identity from the execution environment.

3. The **[App Container Image Discovery](#app-container-image-discovery)** defines: how to take a name like example.com/reduce-worker and translate that into a downloadable image.


## Example Use Case

To provide context to the specs outlined below we will walk through an example.

A user wants to launch a container running three processes.
The three processes the user wants to run are the apps named `example.com/reduce-worker-register`, `example.com/reduce-worker`, and `example.com/reduce-backup`.
First, the executor will check the cache and find that it doesn't have images available for these apps.
So, it will make an HTTPS request to example.com and using the `<meta>` tags there finds that the containers can be found at:

	https://storage-mirror.example.com/reduce-worker.aci
	https://storage-mirror.example.com/worker-backup.aci
	https://storage-mirror.example.com/reduce-worker-register.aci

The executor downloads these three images and puts them into its local on-disk cache.
Then the executor extracts three fresh copies of the images to create instances of the "on-disk app format" and reads the three image manifests to figure out what binaries will need to be executed.

Based on user input, the executor now sets up the necessary cgroups, network interfaces, etc. and runs the `pre-start` event handlers for each app.
Next, it forks the `reduce-worker`, `worker-backup`, and `register` processes in their shared namespaces inside the container, chrooted into their respective root filesystems.

At some point, the container will get some notification that it needs to stop.
The executor will send `SIGTERM` to the processes and after they have exited the `post-stop` event handlers for each app will run.

Now, let's dive into the pieces that took us from three URLs to a running container on our system.

## App Container Image

An *App Container Image* (ACI) contains all files and metadata needed to execute a given app.
In some ways an ACI can be thought of as equivalent to a static binary.

### Image Layout

The on-disk layout of an app container is straightforward.
It includes a *rootfs* with all of the files that will exist in the root of the app, and an *app image manifest* describing the contents of the image and how to execute the app.

```
/manifest
/rootfs
/rootfs/usr/bin/data-downloader
/rootfs/usr/bin/reduce-worker
```

### Image Archives

The ACI file format ("image archive") aims for flexibility and relies on standard and common technologies: HTTP, gpg, tar and gzip.
This set of formats makes it easy to build, host and secure a container using technologies that are widely available and battle-tested.

- Image archives MUST be named with the suffix `.aci`, irrespective of compression/encryption (see below).
- Image archives MUST be a tar formatted file with no duplicate entries.
- Image archives MUST have only two top-level pathnames, `manifest` (a regular file) and `rootfs` (a directory). Image archives with additional files outside of `rootfs` are not valid.
- All files in the image MUST maintain all of their original properties, including timestamps, Unix modes, and extended attributes (xattrs).
- Image archives MAY be compressed with `gzip`, `bzip2`, or `xz`.
- Image archives MAY be encrypted with AES symmetric encryption, after (optional) compression.
- Image archives SHOULD be signed using PGP, the format MUST be ascii-armored detached signature mode.
- Image signatures MUST be named with the suffix `.aci.asc`.

The following example demonstrates the creation of a simple ACI using common command-line tools.
In this case, the ACI is both compressed and encrypted.

```
tar cvf reduce-worker.tar manifest rootfs
gzip reduce-worker.tar -c > reduce-worker.aci
gpg --armor --output reduce-worker.aci.asc --detach-sig reduce-worker.aci
gpg --output reduce-worker.aci --digest-algo sha256 --cipher-algo AES256 --symmetric reduce-worker.aci
```

**Note**: the key distribution mechanism to facilitate image signature validation is not defined here.
Implementations of the app container spec will need to provide a mechanism for users to configure the list of signing keys to trust, or use the key discovery described in [App Container Image Discovery](#app-container-image-discovery).

An example application container image builder is [actool](https://github.com/appc/spec/tree/master/actool).

### Image ID

An image is addressed and verified against the hash of its uncompressed tar file, the _image ID_.
The image ID provides a way to uniquely and globally reference an image, and verify its integrity at any point.
The default digest format is sha512, but all hash IDs in this format are prefixed by the algorithm used (e.g. sha512-a83...).

```
echo sha512-$(sha512sum reduce-worker.tar | awk '{print $1}')
```

### Image Manifest

The [image manifest](#image-manifest-schema) is a [JSON](https://tools.ietf.org/html/rfc4627) file that includes details about the contents of the ACI, and optionally information about how to execute a process inside the ACI's rootfs.
If included, execution details include mount points that should exist, the user, the command args, default cgroup settings and more.
The manifest MAY also define binaries to execute in response to lifecycle events of the main process such as *pre-start* and *post-stop*.

Image manifests MUST be valid JSON located in the file `manifest` in the root of the image archive.
Image manifests MAY specify dependencies, which describe how to assemble the final rootfs from a collection of other images.
As an example, an app might require special certificates to be layered into its filesystem.
In this case, the app can reference the name "example.com/trusted-certificate-authority" as a dependency in the image manifest.
The dependencies are applied in order and each image dependency can overwrite files from the previous dependency.
Execution details specified in image dependencies are ignored.
An optional *path whitelist* can be provided, in which case all non-specified files from all dependencies will be omitted in the final, assembled rootfs.

Image Format TODO

* Define the garbage collection lifecycle of the container filesystem including:
    * Format of app exit code and signal
    * The refcounting plan for resources consumed by the ACE such as volumes
* Define the lifecycle of the container as all exit or first to exit
* Define security requirements for a container. In particular is any isolation of users required between containers? What user does each application run under and can this be root (i.e. "real" root in the host).
* Define how apps are supposed to communicate; can they/do they 'see' each other (a section in the apps perspective would help)?


## App Container Executor

App Containers are a combination of a number of technologies which are not aware of each other.
This specification attempts to define a reasonable subset of steps to accomplish a few goals:

* Creating a filesystem hierarchy in which the app will execute
* Running the app process inside of a combination of resource and namespace isolations
* Executing the application inside of this environment

There are two "perspectives" in this process.
The "*executor*" perspective consists of the steps that the container executor must take to set up the containers. The "*app*" perspective is how the app processes inside the container see the environment.

### Executor Perspective

#### Filesystem Setup

Every execution of an app container should start from a clean copy of the app image.
The simplest implementation will take an application container image (ACI) and extract it into a new directory:

```
cd $(mktemp -d -t temp.XXXX)
mkdir hello
tar xzvf /var/lib/pce/hello.aci -C hello
```

Other implementations could increase performance and de-duplicate data by building on top of overlay filesystems, copy-on-write block devices, or a content-addressed file store.
These details are orthogonal to the runtime environment.

#### Container Runtime Manifest

A container executes one or more apps with shared PID namespace, network namespace, mount namespace, IPC namespace and UTS namespace.
Each app will start chrooted into its own unique read-write rootfs before execution.
The definition of the container is a list of apps that should be launched together, along with isolators that should apply to the entire container.
This is codified in a [Container Runtime Manifest](#container-runtime-manifest-schema).

This example container will use a set of three apps:

| Name                               | Version | Image hash                                      |
|------------------------------------|---------|-------------------------------------------------|
| example.com/reduce-worker          | 1.0.0   | sha512-277205b3ae3eb3a8e042a62ae46934b470e43... |
| example.com/worker-backup          | 1.0.0   | sha512-3e86b59982e49066c5d813af1c2e2579cbf57... |
| example.com/reduce-worker-register | 1.0.0   | sha512-86298e1fdb95ec9a45b5935504e26ec29b8fe... |

#### Volume Setup

Volumes that are specified in the Container Runtime Manifest are mounted into each of the apps via a bind mount.
For example say that the worker-backup and reduce-worker both have a MountPoint named "work".
In this case, the container executor will bind mount the host's `/opt/tenant1/database` directory into the Path of each of the matching "work" MountPoints of the two containers.

#### Network Setup

An App Container must have a [layer 3](http://en.wikipedia.org/wiki/Network_layer) (commonly called the IP layer) network interface, which can be instantiated in any number of ways (e.g. veth, macvlan, ipvlan, device pass-through).
The network interface should be configured with an IPv4/IPv6 address that is reachable from other containers.

#### Logging

Apps should log to stdout and stderr. The container executor is responsible for capturing and persisting the output.

If the application detects other logging options, such as the /run/systemd/system/journal socket, it may optionally upgrade to using those mechanisms.
Note that logging mechanisms other than stdout and stderr are not required by this specification (or tested by the compliance tests).

### Apps Perspective

#### Execution Environment

* **Working directory** defaults to the root of the application image, overridden with "workingDirectory"
* **PATH** `/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin`
* **USER, LOGNAME** username of the user executing this app
* **HOME** home directory of the user
* **SHELL** login shell of the user
* **AC_APP_NAME** name of the application, as defined in the image manifest
* **AC_METADATA_URL** URL where the metadata service for this container can be found

### Isolators

Isolators enforce resource constraints rather than namespacing.
Isolators may be scoped to individual applications, to whole containers, or to both.
Some well known isolators can be verified by the specification.
Additional isolators will be added to this specification over time.

An isolator is a standalone JSON object with only one required field: "name".
All other fields are specific to the isolator.

An executor MAY ignore isolators that it does not understand and run the container without them.
But, an executor MUST make information about which isolators were ignored, enforced or modified available to the user.
An executor MAY implement a "strict mode" where an image cannot run unless all isolators are in place.

### Linux Isolators

These isolators are specific to the Linux kernel and are impossible to represent as a 1-to-1 mapping on other kernels.
The first example is "capabilities" but this will be expanded to include things such as SELinux, SMACK or AppArmor.

#### linux/capabilities-remove-set

* Scope: app

**Parameters:**
* **set** list of capabilities that will be removed from the process's capabilities bounding set

```
"name": "os/linux/capabilities-remove-set",
"value": {
  "set": [
    "CAP_SYS_PTRACE",
  ]
}
```

#### linux/capabilities-retain-set

* Scope: app

**Parameters:**
* **set** list of capabilities that will be retained in the process's capabilities bounding set

```
"name": "os/linux/capabilities-retain-set",
"value": {
  "set": [
    "CAP_KILL",
    "CAP_CHOWN"
  ]
}
```

### Resource Isolators

A resource is something that can be consumed by a container such as memory (RAM), CPU, and network bandwidth. These resource isolators will have a request and limit quantity:

- request will be the guaranteed quantity resources given to the container. Going over the request may result in throttling or denial. If request is omitted, it defaults to limit.

- limit is the cap on the maximum amount of resources that will be made available to the container. If more resources than the limit are consumed, it may be terminated or throttled to no more than the limit.

Limit and requests will always be a resource type's natural base units (e.g., bytes, not MB). These quantities may either be unsuffixed, have suffices (E, P, T, G, M, K, m) or power-of-two suffices (Ei, Pi, Ti, Gi, Mi, Ki). For example, the following represent roughly the same value: 128974848, "129e6", "129M" , "123Mi". Small quantities can be represented directly as decimals (e.g., 0.3), or using milli-units (e.g., "300m").

#### resource/block-bandwidth

* Scope: app/container

**Parameters:**
* **devicePaths** the block devices that this limit will be placed on
* **limit** read/write bytes per second

```
"name": "resouce/block-bandwidth",
"value": {
  "default": true,
  "limit": "2M"
}
```

**TODO**: The default=true behvaior probably covers nearly 80% of all use cases but we need to define a way to target particular devices.

#### resource/block-iops

* Scope: app/container

**Parameters:**
* **default** must be set to true and means that this bandwidth limit applies to all interfaces except localhost by default.
* **limit** read/write input/output operations per second

```
"name": "resource/block-iops",
"value": {
  "default": true,
  "limit": "1000"
}
```

**TODO**: The default=true behvaior probably covers nearly 80% of all use cases but we need to define a way to target particular devices.


#### resource/cpu

* Scope: app/container

**Parameters:**
* **request** milli-cores that are requested
* **limit** milli-cores that can be consumed before the kernel temporarily throttles the process

```
"name": "resource/cpu",
"value": {
  "request": "250",
  "limit": "500"
}
```

**Note**: a milli-core is the milli-seconds/second that the app will be able to run. e.g. 1000 would represent full use of a single CPU core every second.

#### resource/memory

* Scope: app/container

**Parameters:**
* **request** bytes of memory that the app is requesting to use and allocations over this request will be reclaimed in case of contention
* **limit** bytes of memory that the app can allocate before the kernel considers the container out of memory and stops allowing allocations.

```
"name": "resource/memory",
"value": {
  "request": "1G",
  "limit": "2G"
}
```

#### resource/network-bandwidth

* Scope: app/container

**Parameters:**
* **default** must be set to true and means that this bandwidth limit applies to all interfaces except localhost by default.
* **limit** read/write bytes per second

```
"name": "resource/network-bandwidth",
"value": {
  "default": true,
  "limit": "1G"
}
```

**NOTE**: Limits SHOULD NOT apply to localhost communication between apps in a container.
**TODO**: The default=true behvaior probably covers nearly 80% of all use cases but we need to define a network interface tagging spec for appc.

## App Container Image Discovery

An app name has a URL-like structure, for example `example.com/reduce-worker`.
However, there is no scheme on this app name, so it cannot be directly resolved to an app container image URL.
Furthermore, attributes other than the name may be required to unambiguously identify an app (version, OS and architecture).
App Container Image Discovery prescribes a discovery process to retrieve an image based on the app name and these attributes.
Image Discovery is inspired by Go's [remote import paths](https://golang.org/cmd/go/#hdr-Remote_import_paths).

There are three URLs types:
* Image URLs
* Signature URLs
* Public key URLs

Simple and Meta Discovery processes use one or more templates (predefined or derived from various sources) to render Image and Signature URLs (while the Public keys URLs aren't templates).

Note that, to discriminate between the image and its signature, the templates must contain `{ext}` and its values should be either `aci` (for the image) or `aci.asc` (for the signature).

### Simple Discovery

The simple discovery template is:

    https://{name}-{tag}-{os}-{arch}.{ext}

First, try to fetch the app container image by rendering the above template (with `{ext}` rendered to `aci`) and directly retrieving the resulting URL.

For example, given the app name `example.com/reduce-worker`, with tag `1.0.0`, arch `amd64`, and os `linux`, try to retrieve:

    https://example.com/reduce-worker-1.0.0-linux-amd64.aci

If this fails, move on to meta discovery.
If this succeeds, try fetching the signature using the same template but with `{ext}` rendered to `aci.asc`:

    https://example.com/reduce-worker-1.0.0-linux-amd64.aci.asc

Simple discovery doesn't provide a way to discover Public Keys.

### Meta Discovery

If simple discovery fails, then we use HTTPS+HTML meta tags to resolve an app name to a downloadable URL.
For example, if the ACE is looking for `example.com/reduce-worker` it will request:

    https://example.com/reduce-worker?ac-discovery=1

Then inspect the HTML returned for meta tags that have the following format:

```
<meta name="ac-discovery" content="prefix-match url-tmpl">
<meta name="ac-discovery-pubkeys" content="prefix-match url">
```

* `ac-discovery` should contain a URL template that can be rendered to retrieve the ACI or signature
* `ac-discovery-pubkeys` should contain a URL that provides a set of public keys that can be used to verify the signature of the ACI

Some examples for different schemes and URLs:

```
<meta name="ac-discovery" content="example.com https://storage.example.com/{os}/{arch}/{name}-{tag}.{ext}?torrent">
<meta name="ac-discovery" content="example.com hdfs://storage.example.com/{name}-{tag}-{os}-{arch}.{ext}">
<meta name="ac-discovery-pubkeys" content="example.com https://example.com/pubkeys.gpg">
```

The algorithm first ensures that the prefix of the AC Name matches the prefix-match and then if there is a match it will request the equivalent of:

```
curl $(echo "$urltmpl" | sed -e "s/{name}/$appname/" -e "s/{tag}/$tag/" -e "s/{os}/$os/" -e "s/{arch}/$arch/" -e "s/{ext}/$ext/")
```

where _appname_, _tag_, _os_, and _arch_ are set to their respective values for the application, and _ext_ is either `aci` or `aci.asc` for retrieving an app container image or signature respectively.

In our example above this would be:

```
sig: https://storage.example.com/linux/amd64/reduce-worker-1.0.0.aci.asc
aci: https://storage.example.com/linux/amd64/reduce-worker-1.0.0.aci
keys: https://example.com/pubkeys.gpg
```

This mechanism is only used for discovery of contents URLs.

If the first attempt at fetching the discovery URL returns a status code other than `200 OK`, `3xx`, or does not contain any `ac-discovery` meta tags then the next higher path in the name should be tried.
For example if the user has `example.com/project/subproject` and we first try `example.com/project/subproject` but don't find a meta tag then try `example.com/project` then try `example.com`.

All HTTP redirects should be followed when the discovery URL returns a `3xx` status code.

Discovery URLs that require interpolation are [RFC6570](https://tools.ietf.org/html/rfc6570) URI templates.

### Validation

Implementations of the spec should enforce any signature validation rules set in place by the operator.
For example, in a testing environment, signature validation might be disabled, in which case the implementation would omit the signature retrieval.

Implementations must ensure that the name in the Image Manifest in the retrieved ACI matches the initial name used for discovery.

### Authentication

Authentication during the discovery process is optional.
If an attempt at fetching any resource (the initial discovery URL, an app container image, or signature) returns a `401 Unauthorized`, implementations should enact the authentication policy set by the operator.
For example, some implementations might only perform HTTP basic authentication over HTTPS connections.

## App Container Metadata Service

For a variety of reasons, it is desirable to not write files to the filesystem in order to run a container:
* Secrets can be kept outside of the container (such as the identity endpoint specified below)
* Writing files leads to assumptions like a libc environment attempting parse `/etc/hosts`
* The container can be run on top of a cryptographically secured read-only filesystem
* Metadata is a proven system for virtual machines

The app container specification defines an HTTP-based metadata service for providing metadata to containers.

### Metadata Server

The ACE must provide a Metadata server on the address given to the container via the `AC_METADATA_URL` environment variable.

Clients querying any of these endpoints must specify the `Metadata-Flavor: AppContainer` header.

### Container Metadata

Information about the container that this app is executing in.

Retrievable at `$AC_METADATA_URL/acMetadata/v1/container`

| Entry       | Description |
|-------------|-------------|
|annotations/ | Top level annotations from container runtime manifest. |
|manifest     | The container manifest JSON. |
|uid          | The unique execution container uid. |

### App Metadata

Every running process will be able to introspect its App Name via the `AC_APP_NAME` environment variable.
This is necessary to query for the correct endpoint metadata.

Retrievable at `$AC_METADATA_URL/acMetadata/v1/apps/$AC_APP_NAME/`

| Entry         | Description |
|---------------|-------------|
|annotations/   | Annotations from image manifest merged with app annotations from container runtime manifest. |
|image/manifest | The original manifest file of the app. |
|image/id       | Image ID (digest) this app is contained in. |

### Identity Endpoint

As a basic building block for building a secure identity system, the metadata service must provide an HMAC (described in [RFC2104](https://www.ietf.org/rfc/rfc2104.txt)) endpoint for use by the apps in the container.
This gives a cryptographically verifiable identity to the container based on its container unique ID and the container HMAC key, which is held securely by the ACE.

Accessible at `$AC_METADATA_URL/acMetadata/v1/container/hmac`

| Entry | Description |
|-------|-------------|
|sign   | POST a form with content=&lt;object to sign&gt; and retrieve a base64 hmac-sha512 signature as the response body. The metadata service holds onto the secret key as a sort of container TPM. |
|verify | Verify a signature from another container. POST a form with content=&lt;object that was signed&gt;, uid=&lt;uid of the container that generated the signature&gt;, signature=&lt;base64 encoded signature&gt;. Returns 200 OK if the signature passes and 403 Forbidden if the signature check fails. |


## AC Name Type

An AC Name Type is restricted to lowercase characters accepted by the DNS [RFC1123](http://tools.ietf.org/html/rfc1123#page-13) and "/".
An AC Name Type cannot be an empty string and must begin and end with an alphanumeric character.
An AC Name Type will match the following [RE2](https://code.google.com/p/re2/wiki/Syntax) regular expression: `^[a-z0-9]+([-./][a-z0-9]+)*$`

Examples:

* database
* example.com/database
* example.com/ourapp
* sub-domain.example.com/org/product/release

The AC Name Type is used as the primary key for a number of fields in the schemas below.
The schema validator will ensure that the keys conform to these constraints.


## Manifest Schemas

### Image Manifest Schema

JSON Schema for the Image Manifest (app image manifest, ACI manifest), conforming to [RFC4627](https://tools.ietf.org/html/rfc4627)

```
{
    "acKind": "ImageManifest",
    "acVersion": "0.3.0",
    "name": "example.com/reduce-worker",
    "labels": [
        {
            "name": "tag",
            "value": "1.0.0"
        },
        {
            "name": "arch",
            "value": "amd64"
        },
        {
            "name": "os",
            "value": "linux"
        }
    ],
    "app": {
        "exec": [
            "/usr/bin/reduce-worker",
            "--quiet"
        ],
        "user": "100",
        "group": "300",
        "eventHandlers": [
            {
                "exec": [
                    "/usr/bin/data-downloader"
                ],
                "name": "pre-start"
            },
            {
                "exec": [
                    "/usr/bin/deregister-worker",
                    "--verbose"
                ],
                "name": "post-stop"
            }
        ],
        "workingDirectory": "/opt/work",
        "environment": [
            {
                "name": "REDUCE_WORKER_DEBUG",
                "value": "true"
            }
        ],
        "isolators": [
            {
                "name": "resources/cpu",
                "value": {
                    "request": "250",
                    "limit": "500"
                }
            },
            {
                "name": "resource/memory",
                "value": {
                    "request": "1G",
                    "limit": "2G"
                }
            },
            {
                "name": "linux/capabilities-retain-set",
                "value": {
                    "set": ["CAP_NET_BIND_SERVICE", "CAP_SYS_ADMIN"]
                }
            }
        ],
        "mountPoints": [
            {
                "name": "work",
                "path": "/var/lib/work",
                "readOnly": false
            }
        ],
        "ports": [
            {
                "name": "health",
                "port": 4000,
                "protocol": "tcp",
                "socketActivated": true
            }
        ]
    },
    "dependencies": [
        {
            "app": "example.com/reduce-worker-base",
            "imageID": "sha512-...",
            "labels": [
                {
                    "name": "os",
                    "value": "linux"
                },
                {
                    "name": "env",
                    "value": "canary"
                }
            ]
        }
    ],
    "pathWhitelist": [
        "/etc/ca/example.com/crt",
        "/usr/bin/map-reduce-worker",
        "/opt/libs/reduce-toolkit.so",
        "/etc/reduce-worker.conf",
        "/etc/systemd/system/"
    ],
    "annotations": [
        {
            "name": "authors",
            "value": "Carly Container <carly@example.com>, Nat Network <[nat@example.com](mailto:nat@example.com)>"
        },
        {
            "name": "created",
            "value": "2014-10-27T19:32:27.67021798Z"
        },
        {
            "name": "documentation",
            "value": "https://example.com/docs"
        },
        {
            "name": "homepage",
            "value": "https://example.com"
        }
    ]
}
```

* **acKind** (string, required) must be set to "ImageManifest"
* **acVersion** (string, required) represents the version of the schema specification that the manifest implements (string, must be in [semver](http://semver.org/) format)
* **name** (string, required) used as a human readable index to the container image. (string, restricted to the AC Name formatting)
* **labels** (list of objects, optional) used during image discovery and dependency resolution. The listed objects must have two key-value pairs: *name* is restricted to the AC Name formatting and *value* is an arbitrary string. Label names must be unique within the list, and (to avoid confusion with the image's name) cannot be "name". Several well-known labels are defined:
    * **tag** when combined with "name", this should be unique for every build of an app (on a given "os"/"arch" combination).
    * **os**, **arch** can together be considered to describe the syscall ABI this image requires. **arch** is meaningful only if **os** is provided. If one or both values are not provided, the image is assumed to be OS- and/or architecture-independent. Currently supported combinations are listed in the [`types.ValidOSArch`](schema/types/labels.go) variable, which can be updated by an implementation that supports other combinations. The combinations whitelisted by default are (in format `os/arch`): `linux/amd64`, `linux/i386`, `freebsd/amd64`, `freebsd/i386`, `freebsd/arm`, `darwin/x86_64`, `darwin/i386`.
* **app** (object, optional) if present, defines the default parameters that can be used to execute this image as an application.
    * **exec** (list of strings, required) executable to launch and any flags (must be non-empty; the executable must be an absolute path within the app rootfs; ACE can append or override)
    * **user**, **group** (string, required) indicates either the UID/GID or the username/group name the app should run as inside the container (freeform string). If the user or group field begins with a "/", the owner and group of the file found at that absolute path inside the rootfs is used as the UID/GID of the process.
    * **eventHandlers** (list of objects, optional) allows the app to have several hooks based on lifecycle events. For example, you may want to execute a script before the main process starts up to download a dataset or backup onto the filesystem. An eventHandler is a simple object with two fields - an **exec** (array of strings, ACE can append or override), and a **name** (there may be only one eventHandler of a given name), which must be one of:
        * **pre-start** - executed and must exit before the long running main **exec** binary is launched
        * **post-stop** - executed if the main **exec** process is killed. This can be used to cleanup resources in the case of clean application shutdown, but cannot be relied upon in the face of machine failure.
    * **workingDirectory** (string, optional) working directory of the launched application, relative to the application image's root (must be an absolute path, defaults to "/", ACE can override). If the directory does not exist in the application's assembled rootfs (including any dependent images and mounted volumes), the ACE must fail execution.
    * **environment** (list of objects, optional) represents the app's environment variables (ACE can append). The listed objects must have two key-value pairs: **name** and **value**. The **name** must consist solely of letters, digits, and underscores '_' as outlined in [IEEE Std 1003.1-2001](http://pubs.opengroup.org/onlinepubs/009695399/basedefs/xbd_chap08.html). The **value** is an arbitrary string.
    * **isolators** (optional) list of isolation steps that should be applied to the app.
        * **name** is restricted to the [AC Name](#ac-name-type) formatting
    * **mountPoints** (list of objects, optional) locations where a container is expecting external data to be mounted. The listed objects should contain three key-value pairs: the **name** indicates an executor-defined label to look up a mount point, and the **path** stipulates where it should actually be mounted inside the rootfs. The name is restricted to the AC Name Type formatting. **readOnly" should be a boolean indicating whether or not the mount point should be read-only (defaults to "false" if unsupplied).
    * **ports** (list of objects, optional) are protocols and port numbers that the container will be listening on once started. All of the keys in the listed objects are restricted to the AC Name formatting. This information is primarily to help the user find ports that are not well known. It could also optionally be used to limit the inbound connections to the container via firewall rules to only ports that are explicitly exposed.
        * **socketActivated** (boolean, optional) if set to true, the application expects to be [socket activated](http://www.freedesktop.org/software/systemd/man/sd_listen_fds.html) on these ports. The ACE must pass file descriptors using the [socket activation protocol](http://www.freedesktop.org/software/systemd/man/sd_listen_fds.html) that are listening on these ports when starting this container. If multiple apps in the same container are using socket activation then the ACE must match the sockets to the correct apps using getsockopt() and getsockname().
* **dependencies** (list of objects, optional) dependent application images that need to be placed down into the rootfs before the files from this image (if any). The ordering is significant. See [Dependency Matching](#dependency-matching) for how dependencies should be retrieved.
    * **app** (string, required) name of the dependent app container image.
    * **imageID** (string, optional) content hash of the dependency. If provided, the retrieved dependency must match the hash. This can be used to produce deterministic, repeatable builds of an App Image that has dependencies.
    * **labels** (list of objects, optional) a list of the very same form as the aforementioned label objects in the top level ImageManifest. See [Dependency Matching](#dependency-matching) for how these are used.
* **pathWhitelist** (list of strings, optional) complete whitelist of paths that should exist in the rootfs after assembly (i.e. unpacking the files in this image and overlaying its dependencies, in order). Paths that end in slash will ensure the directory is present but empty. This field is only required if the app has dependencies and you wish to remove files from the rootfs before running the container; an empty value means that all files in this image and any dependencies will be available in the rootfs.
* **annotations** (list of objects, optional) any extra metadata you wish to add to the image. Each object has two key-value pairs: the *name* is restricted to the [AC Name](#ac-name-type) formatting and *value* is an arbitrary string. Annotation names must be unique within the list. Annotations can be used by systems outside of the ACE (ACE can override). If you are defining new annotations, please consider submitting them to the specification. If you intend for your field to remain special to your application please be a good citizen and prefix an appropriate namespace to your key names. Recognized annotations include:
    * **created** date on which this container was built (string, must be in [RFC3339](https://www.ietf.org/rfc/rfc3339.txt) format)
    * **authors** contact details of the people or organization responsible for the containers (freeform string)
    * **homepage** URL to find more information on the container (string, must be a URL with scheme HTTP or HTTPS)
    * **documentation** URL to get documentation on this container (string, must be a URL with scheme HTTP or HTTPS)

#### Dependency Matching

Dependency matching is based on a combination of the three different fields of the dependency - **app**, **imageID**, and **labels**.
First, the image discovery mechanism is used to locate a dependency.
If any labels are specified in the dependency, they are passed to the image discovery mechanism, and should be used when locating the image.

If the image discovery process successfully returns an image, it will be compared as follows
If the dependency specification has an image ID, it will be compared against the hash of image returned, and must match.
Otherwise, the labels in the dependency specification are compared against the labels in the retrieved ACI (i.e. in its ImageManifest), and must match.
A label is considered to match if it meets one of three criteria:
- It is present in the dependency specification and present in the dependency's ImageManifest with the same value.
- It is absent from the dependency specification and present in the dependency's ImageManifest, with any value.
This facilitates "wildcard" matching and a variety of common usage patterns, like "noarch" or "latest" dependencies.
For example, an AppImage containing a set of bash scripts might omit both "os" and "arch", and hence could be used as a dependency by a variety of different AppImages.
Alternatively, an AppImage might specify a dependency with no image ID and no "tag" label, and the image discovery mechanism could always retrieve the latest tag of an AppImage

### Container Runtime Manifest Schema

JSON Schema for the Container Runtime Manifest (container manifest), conforming to [RFC4627](https://tools.ietf.org/html/rfc4627)

```
{

    "acVersion": "0.3.0",
    "acKind": "ContainerRuntimeManifest",
    "uuid": "6733C088-A507-4694-AABF-EDBE4FC5266F",
    "apps": [
        {
            "name": "reduce-worker",
            "image": {
                "name": "example.com/reduce-worker",
                "id": "sha512-...",
                "labels": [
                    {
                        "name":  "tag",
                        "value": "1.0.0"
                    }
                ]
            },
            "app": {
                "exec": [
                    "/bin/reduce-worker",
                    "--debug=true",
                    "--data-dir=/mnt/foo"
                ],
                "group": "0",
                "user": "0",
                "mountPoints": [
                    {
                        "name": "work",
                        "path": "/mnt/foo"
                    }
                ]
            },
            "mounts": [
                {"volume": "work", "mountPoint": "work"}
            ]
        },
        {
            "name": "backup",
            "image": {
                "name": "example.com/worker-backup",
                "id": "sha512-...",
                "labels": [
                    {
                        "name": "tag",
                        "value": "1.0.0"
                    }
                ],
                "isolators": [
                    {
                        "name": "resource/memory",
                        "value": {
                            "request": "1G",
                            "limit": "2G"
                        }
                    }
                ],
                "annotations": [
                    {
                        "name": "foo",
                        "value": "bax"
                    }
                ],
                "mounts": [
                    {"volume": "work", "mountPoint": "backup"}
                ]
            }
            "annotations": [
                {
                    "name": "foo",
                    "value": "baz"
                }
            ],
            "mounts": [
                 {"volume": "work", "mountPoint": "backup"}
            ]
        },
        {
            "name": "register",
            "image": {
                "name": "example.com/reduce-worker-register",
                "id": "sha512-...",
                "labels": [
                    {
                        "name": "tag",
                        "value": "1.0.0"
                    }
                ]
            }
        }
    ],
    "volumes": [
        {
            "name": "work",
            "kind": "host",
            "source": "/opt/tenant1/work",
            "readOnly": true,
        }
    ],
    "isolators": [
        {
            "name": "resource/memory",
            "value": {
                "limit": "4G"
            }
        },
    ],
    "annotations": [
        {
           "name": "ip-address",
           "value": "10.1.2.3"
        }
    ]
}
```

* **acVersion** (string, required) represents the version of the schema spec (must be in [semver](http://semver.org/) format)
* **acKind** (string, required) must be set to "ContainerRuntimeManifest"
* **uuid** (string, required) must be an [RFC4122 UUID](http://www.ietf.org/rfc/rfc4122.txt) that represents this instance of the container (must be in [RFC4122](http://www.ietf.org/rfc/rfc4122.txt) format)
* **apps** (list of objects, required) list of apps that will execute inside of this container. Each app object has the following set of key-value pairs:
    * **name** (string, optional) name of the app (restricted to AC Name formatting)
    * **image** (object, required) identifiers of the image providing this app
        * **id** (string, required) content hash of the image that this app will execute inside of (must be of the format "type-value", where "type" is "sha512" and value is the hex encoded string of the hash)
        * **name** (string, optional) name of the image (restricted to AC Name formatting)
        * **labels** (list of objects, optional) additional labels characterizing the image
    * **app** (object, optional) substitute for the app object of the referred image's ImageManifest. See [Image Manifest Schema](#image-manifest-schema) for what the app object contains.
    * **mounts** (list of objects, optional) list of mounts mapping an app mountPoint to a volume. Each mount has the following set of key-value pairs:
      * **volume** (string, required) name of the volume that will fulfill this mount (restricted to the AC Name formatting)
      * **mountPoint** (string, required) name of the app mount point to place the volume on (restricted to the AC Name formatting)
    * **annotations** (list of objects, optional) arbitrary metadata appended to the app. The annotation objects must have a *name* key that has a value that is restricted to the [AC Name](#ac-name-type) formatting and *value* key that is an arbitrary string). Annotation names must be unique within the list. These will be merged with annotations provided by the image manifest when queried via the metadata service; values in this list take precedence over those in the image manifest.
* **volumes** (list of objects, optional) list of volumes which should be mounted into each application's filesystem
    * **name** (string, required) used to map the volume to an app's mountPoint at runtime. (restricted to the AC Name formatting)
    * **kind** (string, required) either "empty" or "host". "empty" fulfills a mount point by ensuring the path exists (writes go to container). "host" fulfills a mount point with a bind mount from a **source**.
    * **source** (string, required if **kind** is "host") absolute path on host to be bind mounted into the container under a mount point.
    * **readOnly** (boolean, optional if **kind** is "host") whether or not the volume should be mounted read only.
* **isolators** (list of objects, optional) list of isolators that will apply to all apps in this container. Each object has two key value pairs: **name** is restricted to the AC Name formatting and **value** can be a freeform string)
* **annotations** (list of objects, optional) arbitrary metadata the executor should make available to applications via the metadata service. Objects must contain two key-value pairs: **name** is restricted to the [AC Name](#ac-name-type) formatting and **value** is an arbitrary string). Annotation names must be unique within the list.
