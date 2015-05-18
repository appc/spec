# App Container Specification

_For version information, see [VERSION](VERSION)_

"App Container" (appc) is a specification describing how applications can be packaged, distributed, and executed in a portable and self-contained way.
The specification defines an image format, image discovery mechanism, and execution environment.
The core goals of the specification include:

* Designing for fast downloads and starts of App Containers
* Ensuring images are cryptographically verifiable and highly cacheable
* Designing for composability and independent implementations
* Using common technologies for cryptography, archiving, compression and transport
* Using the DNS namespace to name and discover images

The specification consists of several key sections:

1. The **[App Container Image](#app-container-image)** defines: how files are assembled together into a single image, verified on download and placed onto disk to be run.

2. **[App Container Image Discovery](#app-container-image-discovery)** defines: how to take a name like `example.com/reduce-worker` and translate that into a downloadable image.

3. The **[App Container Pod](#app-container-pods-pods)** (or "Pod") defines: how one or more App Container Images are grouped into a deployable, executable unit.

4. The **[App Container Executor](#app-container-executor)** defines: how pods are executed and the environment they are run inside (including, for example, filesystem layout, resource constraints, and networking).

    * The [Metadata Service](#app-container-metadata-service) defines how apps within pods can introspect and get a cryptographically verifiable identity from the execution environment.


## Example Use Case

To provide context to the specs outlined below we will walk through an example.

A user wants to launch an "App Container" running three processes.
The three processes the user wants to run are the apps named `example.com/reduce-worker-register`, `example.com/reduce-worker`, and `example.com/reduce-backup`.
First, an executor will make an HTTPS request to example.com and, on inspecting the `<meta>` tags in the returned page, determines that the images can be found at:

	https://storage-mirror.example.com/reduce-worker.aci
	https://storage-mirror.example.com/worker-backup.aci
	https://storage-mirror.example.com/reduce-worker-register.aci

The executor downloads these three images and puts them into its local on-disk cache.
Then the executor extracts three fresh copies of the images to create instances of the "on-disk app format" and reads the three image manifests to figure out what binaries will need to be executed.

Based on user input, the executor now sets up the necessary cgroups, network interfaces, etc. and runs the `pre-start` event handlers for each app.
Next, it forks the `reduce-worker`, `worker-backup`, and `register` processes in their shared namespaces, chrooted into their respective root filesystems.

At some point, the App Container will get some notification that it needs to stop (for example, upon host shutdown).
The executor will send `SIGTERM` to the processes and after they have exited the `post-stop` event handlers for each app will run.

Now, let's dive into the pieces that took us from three URLs to a running App Container on our system.

## App Container Image

An *App Container Image* (ACI) contains all files and metadata needed to execute a given app.
In some ways an ACI can be thought of as equivalent to a static binary.

### Image Layout

The on-disk layout of an App Container Image is straightforward.
It includes a *rootfs* directory with all of the files that will exist in the root of the app, and an *app image manifest* file describing the contents of the image and how to execute the app.

```
/manifest
/rootfs
/rootfs/usr/bin/data-downloader
/rootfs/usr/bin/reduce-worker
```

### Image Archives

The ACI file format ("image archive") aims for flexibility and relies on standard and common technologies: HTTP, gpg, tar and gzip.
This set of formats makes it easy to build, host and secure an ACI using technologies that are widely available and battle-tested.

- Image archives MUST be named with the suffix `.aci`, irrespective of compression/encryption (see below).
- Image archives MUST be a tar formatted file with no duplicate entries.
- Image archives MUST have only two top-level pathnames, `manifest` (a regular file) and `rootfs` (a directory). Image archives with additional files outside of `rootfs` are not valid.
- All files in the image MUST maintain all of their original properties, including timestamps, Unix modes, and extended attributes (xattrs).
- Image archives MAY be compressed with `gzip`, `bzip2`, or `xz`.
- Image archives MAY be encrypted using PGP symmetric encryption with AES cipher, after optional compression.
- Image archives SHOULD be signed using PGP, the format MUST be ascii-armored detached signature mode.
- Image signatures MUST be named with the suffix `.aci.asc`.

**Security Note**: Compressing the image will often change the image size significantly. When encryption is applied after compression, it could lead to information leakage, that would not have been revealed without compression, due to such observable difference. Implementations supporting compression SHOULD include an option to disable compression.

The following example demonstrates the creation of a simple ACI using common command-line tools.
In this case, the ACI is compressed, encrypted, and signed.

```bash
tar cvf reduce-worker.tar manifest rootfs
gzip reduce-worker.tar -c > reduce-worker.aci
gpg --output reduce-worker.aci.out --digest-algo sha256 --cipher-algo AES256 --passphrase sekr3t --symmetric reduce-worker.aci
mv reduce-worker.aci.out reduce-worker.aci
gpg --armor --output reduce-worker.aci.asc --detach-sig reduce-worker.aci
```

**Note**: the key distribution mechanism to facilitate image signature validation is not defined here.
Implementations of the App Container spec will need to provide a mechanism for users to configure the list of signing keys to trust, or use the key discovery described in [App Container Image Discovery](#app-container-image-discovery).

An example App Container Image builder is [actool](https://github.com/appc/spec/tree/master/actool).

### Image ID

An image is addressed and verified against the hash of its uncompressed tar file, known as its _image ID_.
The image ID provides a way to uniquely and globally reference an image, and verify its integrity at any point.
An image ID is canonically represented as a string prefixed by the algorithm used (e.g. sha512-a83...): this format and the allowed hash algorithms are defined by the [Image ID Type](#image-id-type).

```bash
echo sha512-$(sha512sum reduce-worker.tar | awk '{print $1}')
```

### Image Manifest

The [image manifest](#image-manifest-schema) is a [JSON](https://tools.ietf.org/html/rfc4627) file that includes details about the contents of the ACI, and optionally information about how to execute a process inside the ACI's rootfs.
If included, execution details include mount points that must exist, the user, the command args, default cgroup settings and more.
The manifest MAY also define binaries to execute in response to lifecycle events of the main process such as *pre-start* and *post-stop*.

Image manifests MUST be valid JSON located in the file `manifest` in the root of the image archive.
Image manifests MAY specify dependencies, which describe how to assemble the final rootfs from a collection of other images.
As an example, an app might require special certificates to be layered into its filesystem.
In this case, the app can reference the name "example.com/trusted-certificate-authority" as a dependency in the image manifest.
The dependencies are applied in order and each image dependency can overwrite files from the previous dependency.
Execution details specified in image dependencies are ignored.
An optional *path whitelist* can be provided, in which case all non-specified files from all dependencies will be omitted in the final, assembled rootfs.

## App Container Pods (pods)

The deployable, executable unit in the App Container specification is the **pod**.
A **pod** is a list of apps that will be launched together inside a shared execution context.
The execution context can be defined as the conjunction of several Linux namespaces (or equivalents on other operating systems):

- PID namespace (apps within the pod can see and signal each other's processes)
- network namespace (apps within the pod have access to the same IP and port space)
- IPC namespace (apps within the pod can use SystemV IPC or POSIX message queues to communicate)
- UTS namespace (apps within the pod share a hostname)

The context MAY include shared volumes, which are defined at the pod level and must be made available in each app's filesystem.
The context MAY additionally consist of one or more [isolators](#isolators).

The definition of the **pod** - namely, the list of constituent apps, and any isolators that apply to the entire pod - is codified in a [Pod Manifest](#pod-manifest-schema).
Pod Manifests can serve the role of both _deployable template_ and _runtime manifest_: a template can be a candidate for a series of transformations before execution.
For example, a Pod Manifest might reference an app with a label requirement of `version=latest`, which another tool might subsequently resolve to a specific version.
Another example would be that volumes are "late-bound" by the executor; alternatively, an executor might add annotations.
Pod Manifests also provide the ability to override application execution parameters for their constituent ACIs (i.e. the `app` section of the respective Image Manifests).

A Pod Manifest must be fully resolved (_reified_) before execution.
Specifically, a Pod Manifest must have all `mountPoint`s satisfied by `volume`s, and must reference all applications deterministically (by image ID).
At runtime, the reified Pod Manifest is exposed to applications through the [Metadata Service](#app-container-metadata-service).

## App Container Executor

The **App Container Executor** defines the process by which applications contained in ACIs are executed.
There are two "perspectives" in this process.
The "*executor*" perspective consists of the steps that the App Container Executor (ACE) must take to set up the environment for the pod and applications.
The "*app*" perspective is how the app processes inside the pod see the environment.

This example pod will use a set of three apps:

| Name                               | Version | Image hash                                      |
|------------------------------------|---------|-------------------------------------------------|
| example.com/reduce-worker          | 1.0.0   | sha512-277205b3ae3eb3a8e042a62ae46934b470e43... |
| example.com/worker-backup          | 1.0.0   | sha512-3e86b59982e49066c5d813af1c2e2579cbf57... |
| example.com/reduce-worker-register | 1.0.0   | sha512-86298e1fdb95ec9a45b5935504e26ec29b8fe... |

#### Pod UUID

Each pod much be assigned an [RFC4122 UUID](http://www.ietf.org/rfc/rfc4122.txt). 
The UUID serves as a canonical reference to a pod within a given administrative domain.
In this context, an administrative domain is linked to the scope of the associated [Metadata Service](#app-container-metadata-service).
For example, given a metadata service that is federated across a geographical cluster of systems, the pod UUID is uniquely scoped to the same cluster.
This UUID is exposed to the pod through the [Metadata Service](#app-container-metadata-service).

#### Filesystem Setup

Each app in a pod will start chrooted into its own unique read-write filesystem before execution.

An app's filesystem must be *rendered* in an empty directory by the following process (or equivalent):
- The `rootfs` contained in the ACI is extracted
- If the ACI contains a non-empty `dependencies` field in its `ImageManifest`, the `rootfs` of each dependent image is extracted, in the order in which they are listed
- If the ACI contains a non-empty `pathWhitelist` field in its `ImageManifest`, *all* paths not in the whitelist must be removed

Every execution of an app MUST start from a clean copy of this rendered filesystem.

The simplest implementation will take an ACI (with no dependencies) and extract it into a new directory:

```bash
cd $(mktemp -d -t temp.XXXX)
mkdir hello
tar xzvf /var/lib/pce/hello.aci -C hello
```

Other implementations could increase performance and de-duplicate data by building on top of overlay filesystems, copy-on-write block devices, or a content-addressed file store.
These details are orthogonal to the runtime environment.

#### Volume Setup

Volumes that are specified in the Pod Manifest are mounted into each of the apps via a bind mount (or equivalent).
For example, say that the worker-backup and reduce-worker both have a `mountPoint` named "work".
In this case, the executor will bind mount the host's `/opt/tenant1/work` directory into the `path` of each of the matching "work" `mountPoint`s of the two app filesystems.

#### Network Setup

A Pod must have a loopback network interface and zero or more [layer 3](http://en.wikipedia.org/wiki/Network_layer) (commonly called the IP layer) network interfaces, which can be instantiated in any number of ways (e.g. veth, macvlan, ipvlan, device pass-through).
Each network interface MUST be configured with one or more IPv4 and/or IPv6 addresses.

#### Logging

Apps SHOULD log to stdout and stderr.  The ACE is responsible for capturing and persisting this output.

If the application detects other logging options, such as the `/run/systemd/system/journal` socket, it may optionally upgrade to using those mechanisms.
Note that logging mechanisms other than stdout and stderr are not required by this specification (and are not tested during compliancy verifications).

### Apps Perspective

#### Execution Environment

The following environment variables MUST be set for each application's main process and any lifecycle processes:

* **PATH** `/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin`
* **AC_APP_NAME** name of the application, as defined in the image manifest
* **AC_METADATA_URL** URL where the [metadata service](#app-container-metadata-service) for this pod can be found.

An executor MAY set additional environment variables for the application processes.

Additionally, processes must have their **working directory** set to the value of the application's **workingDirectory** option, if specified, or the root of the application image by default.

### Isolators

Isolators enforce resource constraints rather than namespacing.
Isolators may be scoped to individual applications, to whole pods, or to both.
Any isolators applied to the pod will _bound_ any individual isolators applied to applications within the pod.

Some well known isolators can be verified by the specification.
Additional isolators will be added to this specification over time.

An executor MAY ignore isolators that it does not understand and run the pod without them.
But, an executor MUST make information about which isolators were ignored, enforced or modified available to the user.
An executor MAY implement a "strict mode" where an image cannot run unless all isolators are in place.

### Linux Isolators

These isolators are specific to the Linux kernel and are impossible to represent as a 1-to-1 mapping on other kernels.
The first example is "capabilities" but this will be expanded to include things such as SELinux, SMACK or AppArmor.

#### os/linux/capabilities-remove-set

* Scope: app

**Parameters:**

* **set** list of capabilities that will be removed from the process's capabilities bounding set, all others will be included.

```json
"name": "os/linux/capabilities-remove-set",
"value": {
  "set": [
    "CAP_SYS_PTRACE"
  ]
}
```

#### os/linux/capabilities-retain-set

* Scope: app

**Parameters:**

* **set** list of capabilities that will be retained in the process's capabilities bounding set, all others will be removed

```json
"name": "os/linux/capabilities-retain-set",
"value": {
  "set": [
    "CAP_KILL",
    "CAP_CHOWN"
  ]
}
```

### Resource Isolators

A _resource_ is something that can be consumed by an application (app) or group of applications (pod), such as memory (RAM), CPU, and network bandwidth.
Resource isolators have a *request* and *limit* quantity:

- **request** is the minimum amount of a resource guaranteed to be available to the app/pod.
If the app/pod attempts to consume a resource in excess of its request, it may be throttled or denied.
If **request** is omitted, it defaults to the value of **limit**.

- **limit** is the maximum amount of a resource available to the app/pod.
If the app/pod consumes a resource in excess of its limit, it must be terminated or throttled to no more than the limit.

Limit and request quantities must always be represented internally (i.e. for encoding and any processing) as an integer value (i.e. NOT floating point) in a resource type's natural base units (e.g., bytes, not megabytes or gigabytes).
For convenience, when specified by users quantities may either be unsuffixed, have metric suffices (E, P, T, G, M, K) or binary (power-of-two) suffices (Ei, Pi, Ti, Gi, Mi, Ki).
For example, the following strings represent the same value: "128974848", "125952Ki", "123Mi".
Sub-units (e.g. decimals, "0.3", or milli-units, "300m") are NOT permissible.

#### resource/block-bandwidth

* Scope: app/pod

**Parameters:**

* **default** must be set to true and means that this limit applies to all block devices by default
* **limit** read/write bytes per second

```json
"name": "resource/block-bandwidth",
"value": {
  "default": true,
  "limit": "2M"
}
```

#### resource/block-iops

* Scope: app/pod

**Parameters:**

* **default** must be set to true and means that this limit applies to all block devices by default
* **limit** read/write input/output operations per second

```json
"name": "resource/block-iops",
"value": {
  "default": true,
  "limit": "1000"
}
```

#### resource/cpu

* Scope: app/pod

**Parameters:**

* **request** milli-cores that are requested
* **limit** milli-cores that can be consumed before the kernel temporarily throttles the process

```json
"name": "resource/cpu",
"value": {
  "request": "250",
  "limit": "500"
}
```

**Note**: a milli-core is the milli-seconds/second that the app/pod will be able to run. e.g. 1000 would represent full use of a single CPU core every second.

#### resource/memory

* Scope: app/pod

**Parameters:**

* **request** bytes of memory that the app/pod is requesting to use and allocations over this request will be reclaimed in case of contention
* **limit** bytes of memory that the app can allocate before the kernel considers the app/pod out of memory and stops allowing allocations.

```json
"name": "resource/memory",
"value": {
  "request": "1G",
  "limit": "2G"
}
```

#### resource/network-bandwidth

* Scope: app/pod

**Parameters:**

* **default** must be set to true and means that this bandwidth limit applies to all interfaces (except localhost) by default.
* **limit** read/write bytes per second

```json
"name": "resource/network-bandwidth",
"value": {
  "default": true,
  "limit": "1G"
}
```

**NOTE**: Network limits MUST NOT apply to localhost communication between apps in a pod.

## App Container Image Discovery

An app name has a URL-like structure, for example `example.com/reduce-worker`.
However, there is no scheme on this app name, so it cannot be directly resolved to an App Container Image URL.
Furthermore, attributes other than the name may be required to unambiguously identify an app (version, OS and architecture).
App Container Image Discovery prescribes a discovery process to retrieve an image based on the app name and these attributes.
Image Discovery is inspired by Go's [remote import paths](https://golang.org/cmd/go/#hdr-Remote_import_paths).

There are three URLs types:

* Image URLs
* Signature URLs
* Public key URLs

Simple and Meta Discovery processes use one or more templates (predefined or derived from various sources) to render Image and Signature URLs (while the Public keys URLs aren't templates).

Note that, to discriminate between the image and its signature, the templates must contain `{ext}` and its values MUST be either `aci` (for the image) or `aci.asc` (for the signature).

### Simple Discovery

The simple discovery template is:

    https://{name}-{version}-{os}-{arch}.{ext}

First, try to fetch the App Container Image by rendering the above template (with `{ext}` rendered to `aci`) and directly retrieving the resulting URL.

For example, given the app `{name}`: `example.com/reduce-worker`, with `{version}`: `1.0.0`, `{arch}`: `amd64`, and `{os}`: `linux`, try to retrieve:

    https://example.com/reduce-worker-1.0.0-linux-amd64.aci

If this fails, move on to meta discovery.
If this succeeds, try fetching the signature using the same template but with `{ext}` rendered to `aci.asc`:

    https://example.com/reduce-worker-1.0.0-linux-amd64.aci.asc

Simple discovery does not provide a way to discover Public Keys.

### Meta Discovery

If simple discovery fails, then we use HTTPS+HTML `meta` tags retrieved from a "discovery URL" to resolve an app name to downloadable URLs.

The template for the discovery URL is:

    https://{name}?ac-discovery=1

For example, if the client is looking for `example.com/reduce-worker` it will request:

    https://example.com/reduce-worker?ac-discovery=1

then inspect the HTML returned for `meta` tags that have the following format:

```html
<meta name="ac-discovery" content="prefix-match url-tmpl">
<meta name="ac-discovery-pubkeys" content="prefix-match url">
```

* `ac-discovery` MUST contain a URL template that can be rendered to retrieve the ACI or associated signature
* `ac-discovery-pubkeys` SHOULD contain a URL that provides a set of public keys that can be used to verify the signature of the ACI

Some examples for different schemes and URLs:

```html
<meta name="ac-discovery" content="example.com https://storage.example.com/{os}/{arch}/{name}-{version}.{ext}">
<meta name="ac-discovery" content="example.com hdfs://storage.example.com/{name}-{version}-{os}-{arch}.{ext}">
<meta name="ac-discovery-pubkeys" content="example.com https://example.com/pubkeys.gpg">
```

When evaluating `ac-discovery` tags, the client MUST first ensure that the prefix of the [AC Name](#ac-name-type) being discovered matches the prefix-match, and if so it MUST perform a simple template substitution to determine the URL at which the resource can be retrieved - the effective equivalent of:

```bash
urltmpl="https://{name}-{version}-{os}-{arch}.{ext}"
curl $(echo "$urltmpl" | sed -e "s/{name}/$appname/" -e "s/{version}/$version/" -e "s/{os}/$os/" -e "s/{arch}/$arch/" -e "s/{ext}/$ext/")
```

where _appname_, _version_, _os_, and _arch_ are set to their respective values for the application, and _ext_ is either `aci` or `aci.asc` for retrieving an App Container Image or signature respectively.

Note that multiple `ac-discovery` tags MAY be returned for a given prefix-match (for example, with different scheme names representing different transport mechanisms).
In this case, the client implementation MAY choose which to use at its own discretion.
Public discovery implementations SHOULD always provide at least one HTTPS URL template.

In our example above, using the HTTPS URL template, the client would attempt to retrieve the following URLs:

```
Signature: 	https://storage.example.com/linux/amd64/reduce-worker-1.0.0.aci.asc
ACI: 		https://storage.example.com/linux/amd64/reduce-worker-1.0.0.aci
Keys: 		https://example.com/pubkeys.gpg
```

If the first attempt at fetching the initial discovery URL returns a `4xx` status code or does not contain any `ac-discovery` meta tags then attempt the parent path in the `name`.
For example if the user has `example.com/project/subproject` and we first try `example.com/project/subproject` but do not discover a `meta` tag then try `example.com/project` and then try `example.com`.

All HTTP redirects MUST be followed when the discovery URL returns a `3xx` status code.

Discovery URLs that require interpolation are [RFC6570](https://tools.ietf.org/html/rfc6570) URI templates.

### Validation

Implementations of the spec are responsible for enforcing any signature validation rules set in place by the operator.
For example, in a testing environment, signature validation might be disabled, in which case the implementation would omit the signature retrieval.

Implementations must ensure that the name in the Image Manifest in the retrieved ACI matches the initial name used for discovery.

### Authentication

Authentication during the discovery process is optional.
If an attempt at fetching any resource (the initial discovery URL, an App Container Image, or signature) returns a `401 Unauthorized`, implementations should enact the authentication policy set by the operator.
For example, some implementations might only perform HTTP basic authentication over HTTPS connections.

## App Container Metadata Service

For a variety of reasons, it is desirable to not write files to the filesystem in order to run an App Container:
* Secrets can be kept outside of the app
* The app can be run on top of a cryptographically secured read-only filesystem
* Metadata is a proven system for virtual machines

The App Container specification defines an HTTP-based metadata service for providing metadata to applications, as well as an [identity endpoint](#identity-endpoint).

### Metadata Service

The ACE must provide a Metadata service on the address given to the applications via the `AC_METADATA_URL` [environment variable](#execution-environment).

ACE implementations SHOULD embed an authorization token in `AC_METADATA_URL`, which provides a means for the metadata service to uniquely and securely identify a pod.
For example, `AC_METADATA_URL` passed to a pod could be set to `https://10.0.0.1:8888/Y4vFeVZzKM2T9rwkpWHfqXuGsNjS6O5c` with the path portion acting as a token.
Since the token is used by the Metadata Service to authenticate the pod's identity, it SHOULD have no fewer than 128 bits of entropy (i.e. size of UUID), and SHOULD NOT be easily guessable (e.g. the pod UUID should not be used).


[UUIDs](#pod-uuid) assigned to pods MUST be unique for the administrative domain of the metadata service.

### Pod Metadata

Information about the pod that this app is executing in.

Retrievable at `$AC_METADATA_URL/acMetadata/v1/pod`

| Entry       | Description |
|-------------|-------------|
|annotations/ | Top level annotations from Pod Manifest. |
|manifest     | Fully-reified Pod Manifest JSON. |
|uuid         | Pod UUID. |

### App Metadata

Every running process will be able to introspect its App Name via the `AC_APP_NAME` environment variable.
This is necessary to query for the correct endpoint metadata.

Retrievable at `$AC_METADATA_URL/acMetadata/v1/apps/$AC_APP_NAME/`

| Entry         | Description |
|---------------|-------------|
|annotations/   | Annotations from Image Manifest merged with app annotations from Pod Manifest. |
|image/manifest | Original Image Manifest of the app. |
|image/id       | Image ID (digest) this app is contained in. |

### Identity Endpoint

As a basic building block for building a secure identity system, the metadata service must provide an HMAC (described in [RFC2104](https://www.ietf.org/rfc/rfc2104.txt)) endpoint for use by the apps in the pod.
This gives a cryptographically verifiable identity to the pod based on its unique ID and the pod HMAC key, which is held securely by the ACE.

Accessible at `$AC_METADATA_URL/acMetadata/v1/pod/hmac`

| Entry | Description |
|-------|-------------|
|sign   | POST a form with content=&lt;object to sign&gt; and retrieve a base64 hmac-sha512 signature as the response body. The metadata service holds onto the secret key as a sort of pod TPM. |
|verify | Verify a signature from another pod. POST a form with content=&lt;object that was signed&gt;, uuid=&lt;uuid of the pod that generated the signature&gt;, signature=&lt;base64 encoded signature&gt;. Returns 200 OK if the signature passes and 403 Forbidden if the signature check fails. |


## Types

### AC Name Type

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


### AC Kind Type

An AC Kind cannot be an empty string and must be alphanumeric characters.
An AC Kind value matching defined kinds, will have defined compatibility.
There is no expected compatibility with undefined AC Kinds.

Defined Kinds:

* `ImageManifest`
* `PodManifest`


### AC Version Type

The App Container specification aims to follow semantic versioning and retain forward and backwards compatibility within major versions.
For example, if an implementation is compliant against version 1.0.1 of the spec, it is compatible with the complete 1.x series.

The version of the App Container specification and associated tooling is recorded in [VERSION](https://github.com/appc/spec/blob/master/VERSION), and is otherwise denoted in the [release version](https://github.com/appc/spec/releases) or git version control tag. 

An AC Version must reference a tagged version of the App Container specification, not exceeding the version of its greatest compliance.
An AC Version for [Image Manifest](#image-manifest-schema) and [Pod Manifest](#pod-manifest-schema) schemas must be compatible on major AC version series.
An AC Version cannot be an empty string and must be in [SemVer v2.0.0](http://semver.org/spec/v2.0.0.html) format.


### Image ID Type

An Image ID Type must be a string of the format "hash-value", where "hash" is the hash algorithm used and "value" is the hex encoded string of the digest.
Currently the only permitted hash algorithm is `sha512`.


### Isolator Type

An Isolator Type must be a JSON object with two required fields: "name" and "value".
"name" must be a string restricted to [AC Name](#ac-name-type) formatting.
"value" may be an arbitrary JSON value.


## Manifest Schemas

### Image Manifest Schema

JSON Schema for the Image Manifest (app image manifest, ACI manifest), conforming to [RFC4627](https://tools.ietf.org/html/rfc4627)

```json
{
    "acKind": "ImageManifest",
    "acVersion": "0.5.2",
    "name": "example.com/reduce-worker",
    "labels": [
        {
            "name": "version",
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
                "name": "resource/cpu",
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
                "name": "os/linux/capabilities-retain-set",
                "value": {
                    "set": ["CAP_NET_BIND_SERVICE"]
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
            },
            {
                "name": "ftp-data",
                "port": 20000,
                "count": 1000,
                "protocol": "tcp"
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

* **acKind** (string, required) must be an [AC Kind](#ac-kind-type) of value "ImageManifest"
* **acVersion** (string, required) represents the version of the schema specification [AC Version Type](#ac-version-type)
* **name** (string, required) a human-readable name for this App Container Image (string, restricted to the [AC Name](#ac-name-type) formatting). This is not expected to be unique (see the **version** label) but SHOULD have a URL-like structure to facilitate **[App Container Image Discovery](#app-container-image-discovery)**. If this image is resolved through the discovery process, this field MUST match the name used for discovery.
* **labels** (list of objects, optional) used during image discovery and dependency resolution. The listed objects must have two key-value pairs: *name* is restricted to the [AC Name](#ac-name-type) formatting and *value* is an arbitrary string. Label names must be unique within the list, and (to avoid confusion with the image's name) cannot be "name". Several well-known labels are defined:
    * **version** when combined with "name", this SHOULD be unique for every build of an app (on a given "os"/"arch" combination).
    * **os**, **arch** can together be considered to describe the syscall ABI this image requires. **arch** is meaningful only if **os** is provided. If one or both values are not provided, the image is assumed to be OS- and/or architecture-independent. Currently supported combinations are listed in the [`types.ValidOSArch`](schema/types/labels.go) variable, which can be updated by an implementation that supports other combinations. The combinations whitelisted by default are (in format `os/arch`): `linux/amd64`, `linux/i386`, `freebsd/amd64`, `freebsd/i386`, `freebsd/arm`, `darwin/x86_64`, `darwin/i386`. See the [Operating System spec](OS-SPEC.md) for the environment apps can expect to run in given a known **os** label.
* **app** (object, optional) if present, defines the default parameters that can be used to execute this image as an application.
    * **exec** (list of strings, required) executable to launch and any flags (must be non-empty; the executable must be an absolute path within the app rootfs; ACE can append or override).  These strings are not evaluated in any way and environment variables are not substituted.
    * **user**, **group** (string, required) indicates either the username/group name or the UID/GID the app is to be run as (freeform string). The user and group values may be all numbers to indicate a UID/GID, however it is possible on some systems (POSIX) to have usernames that are all numerical. The user and group values will first be resolved using the image's own `/etc/passwd` or `/etc/group`. If no valid matches are found, then if the string is all numerical, it shall be converted to an integer and used as the UID/GID. If the user or group field begins with a "/", the owner and group of the file found at that absolute path inside the rootfs is used as the UID/GID of the process. Example values for the fields include `root`, `1000`, or `/usr/bin/ping`.
    * **eventHandlers** (list of objects, optional) allows the app to have several hooks based on lifecycle events. For example, you may want to execute a script before the main process starts up to download a dataset or backup onto the filesystem. An eventHandler is a simple object with two fields - an **exec** (array of strings, ACE can append or override), and a **name** (there may be only one eventHandler of a given name), which must be one of:
        * **pre-start** - executed and must exit before the long running main **exec** binary is launched
        * **post-stop** - executed if the main **exec** process is killed. This can be used to cleanup resources in the case of clean application shutdown, but cannot be relied upon in the face of machine failure.
    * **workingDirectory** (string, optional) working directory of the launched application, relative to the application image's root (must be an absolute path, defaults to "/", ACE can override). If the directory does not exist in the application's assembled rootfs (including any dependent images and mounted volumes), the ACE must fail execution.
    * **environment** (list of objects, optional) represents the app's environment variables (ACE can append). The listed objects must have two key-value pairs: **name** and **value**. The **name** must consist solely of letters, digits, and underscores '_' as outlined in [IEEE Std 1003.1-2001](http://pubs.opengroup.org/onlinepubs/009695399/basedefs/xbd_chap08.html). The **value** is an arbitrary string. These values are not evaluated in any way, and no substitutions are made.
    * **isolators** (list of objects of type [Isolator](#isolator-type), optional) list of isolation steps that SHOULD be applied to the app.
    * **mountPoints** (list of objects, optional) locations where an app is expecting external data to be mounted. The listed objects contain the following key-value pairs: the **name** indicates an executor-defined label to look up a mount point, and the **path** stipulates where it is to be mounted inside the rootfs. The name is restricted to the [AC Name](#ac-name-type) Type formatting. **readOnly** is a boolean indicating whether or not the mount point will be read-only (defaults to "false" if unsupplied).
    * **ports** (list of objects, optional) are protocols and port numbers that the app will be listening on once started. All of the keys in the listed objects are restricted to the [AC Name](#ac-name-type) formatting. This information is to help the user discover the listening ports of the application and to specify the ports that can be exposed on the host. It could also optionally be used to limit the inbound connections to the container via firewall rules to only ports that are explicitly exposed.
        * **count** (integer, optional, defaults to 1) specifies a range of ports, starting with "port" and ending with "port" + "count" - 1.
        * **socketActivated** (boolean, optional, defaults to "false" if unsupplied) if set to true, the application expects to be [socket activated](http://www.freedesktop.org/software/systemd/man/sd_listen_fds.html) on these ports. The ACE must pass file descriptors using the [socket activation protocol](http://www.freedesktop.org/software/systemd/man/sd_listen_fds.html) that are listening on these ports when starting this app. If multiple apps in the same pod are using socket activation then the ACE must match the sockets to the correct apps using getsockopt() and getsockname().
* **dependencies** (list of objects, optional) dependent application images that need to be placed down into the rootfs before the files from this image (if any). The ordering is significant. See [Dependency Matching](#dependency-matching) for how dependencies are retrieved.
    * **app** (string, required) name of the dependent App Container Image.
    * **imageID** (string of type [Image ID](#image-id-type), optional) content hash of the dependency. If provided, the retrieved dependency must match the hash. This can be used to produce deterministic, repeatable builds of an App Image that has dependencies.
    * **labels** (list of objects, optional) a list of the very same form as the aforementioned label objects in the top level ImageManifest. See [Dependency Matching](#dependency-matching) for how these are used.
* **pathWhitelist** (list of strings, optional) whitelist of absolute paths that will exist in the app's rootfs after rendering. This must be a complete and absolute set. An empty list is equivalent to an absent value and means that all files in this image and any dependencies will be available in the rootfs.
* **annotations** (list of objects, optional) any extra metadata you wish to add to the image. Each object has two key-value pairs: the *name* is restricted to the [AC Name](#ac-name-type) formatting and *value* is an arbitrary string. Annotation names must be unique within the list. Annotations can be used by systems outside of the ACE (ACE can override). If you are defining new annotations, please consider submitting them to the specification. If you intend for your field to remain special to your application please be a good citizen and prefix an appropriate namespace to your key names. Recognized annotations include:
    * **created** date on which the image was built (string, must be in [RFC3339](https://www.ietf.org/rfc/rfc3339.txt) format)
    * **authors** contact details of the people or organization responsible for the image (freeform string)
    * **homepage** URL to find more information on the image (string, must be a URL with scheme HTTP or HTTPS)
    * **documentation** URL to get documentation on the image (string, must be a URL with scheme HTTP or HTTPS)

#### Dependency Matching

Dependency matching is based on a combination of the three different fields of the dependency - **app**, **imageID**, and **labels**.
First, the image discovery mechanism is used to locate a dependency.
If any labels are specified in the dependency, they are passed to the image discovery mechanism, and will be used when locating the image.

If the image discovery process successfully returns an image, it will be compared as follows.
If the dependency specification has an image ID, it will be compared against the hash of image returned, and must match.
Otherwise, the labels in the dependency specification are compared against the labels in the retrieved ACI (i.e. in its ImageManifest), and must match.
A label is considered to match if it meets one of two criteria:
- It is present in the dependency specification and present in the dependency's ImageManifest with the same value.
- It is absent from the dependency specification and present in the dependency's ImageManifest, with any value.
This facilitates "wildcard" matching and a variety of common usage patterns, like "noarch" or "latest" dependencies.
For example, an AppImage containing a set of bash scripts might omit both "os" and "arch", and hence could be used as a dependency by a variety of different AppImages.
Alternatively, an AppImage might specify a dependency with no image ID and no "version" label, and the image discovery mechanism could always retrieve the latest version of an AppImage

### Pod Manifest Schema

JSON Schema for the Pod Manifest, conforming to [RFC4627](https://tools.ietf.org/html/rfc4627)

```json
{
    "acVersion": "0.5.2",
    "acKind": "PodManifest",
    "apps": [
        {
            "name": "reduce-worker",
            "image": {
                "name": "example.com/reduce-worker",
                "id": "sha512-...",
                "labels": [
                    {
                        "name":  "version",
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
                        "name": "version",
                        "value": "1.0.0"
                    }
                ]
            },
            "app": {
                "exec": [
                    "/bin/reduce-backup"
                ],
                "group": "0",
                "user": "0",
                "mountPoints": [
                    {
                        "name": "backup",
                        "path": "/mnt/bar"
                    }
                ],
                "isolators": [
                    {
                        "name": "resource/memory",
                        "value": {"limit": "1G"}
                    }
                ]
            },
            "mounts": [
                {"volume": "work", "mountPoint": "backup"}
            ],
            "annotations": [
                {
                    "name": "foo",
                    "value": "baz"
                }
            ]
        },
        {
            "name": "register",
            "image": {
                "name": "example.com/reduce-worker-register",
                "id": "sha512-...",
                "labels": [
                    {
                        "name": "version",
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
            "readOnly": true
        }
    ],
    "isolators": [
        {
            "name": "resource/memory",
            "value": {
                "limit": "4G"
            }
        }
    ],
    "annotations": [
        {
           "name": "ip-address",
           "value": "10.1.2.3"
        }
    ],
    "ports": [
        {
            "name": "ftp",
            "hostPort": 2121
        }
    ]
}
```

* **acVersion** (string, required) represents the version of the schema specification [AC Version Type](#ac-version-type)
* **acKind** (string, required) must be an [AC Kind](#ac-kind-type) of value "PodManifest"
* **apps** (list of objects, required) list of apps that will execute inside of this pod. Each app object has the following set of key-value pairs:
    * **name** (string, required) name of the app (restricted to [AC Name](#ac-name-type) formatting). This is used to identify an app within a pod, and hence MUST be unique within the list of apps. This may be different from the name of the referenced image (see below); in this way, a pod can have multiple apps using the same underlying image.
    * **image** (object, required) identifiers of the image providing this app
        * **id** (string of type [Image ID](#image-id-type), required) content hash of the image that this app will execute inside of
        * **name** (string, optional) name of the image (restricted to [AC Name](#ac-name-type) formatting)
        * **labels** (list of objects, optional) additional labels characterizing the image
    * **app** (object, optional) substitute for the app object of the referred image's ImageManifest. See [Image Manifest Schema](#image-manifest-schema) for what the app object contains.
    * **mounts** (list of objects, optional) list of mounts mapping an app mountPoint to a volume. Each mount has the following set of key-value pairs:
      * **volume** (string, required) name of the volume that will fulfill this mount (restricted to the [AC Name](#ac-name-type) formatting)
      * **mountPoint** (string, required) name of the app mount point to place the volume on (restricted to the [AC Name](#ac-name-type) formatting)
    * **annotations** (list of objects, optional) arbitrary metadata appended to the app. The annotation objects must have a *name* key that has a value that is restricted to the [AC Name](#ac-name-type) formatting and *value* key that is an arbitrary string). Annotation names must be unique within the list. These will be merged with annotations provided by the image manifest when queried via the metadata service; values in this list take precedence over those in the image manifest.
* **volumes** (list of objects, optional) list of volumes which will be mounted into each application's filesystem
    * **name** (string, required) used to map the volume to an app's mountPoint at runtime. (restricted to the [AC Name](#ac-name-type) formatting)
    * **kind** (string, required) either "empty" or "host". "empty" fulfills a mount point by ensuring the path exists (i.e., writes go to the app's chroot). "host" fulfills a mount point with a bind mount from a **source**.
    * **source** (string, required if **kind** is "host") absolute path on host to be bind mounted under a mount point in each app's chroot.
    * **readOnly** (boolean, optional if **kind** is "host", defaults to "false" if unsupplied) whether or not the volume will be mounted read only.
* **isolators** (list of objects of type [Isolator](#isolator-type), optional) list of isolation steps that will apply to this pod.
* **annotations** (list of objects, optional) arbitrary metadata the executor will make available to applications via the metadata service. Objects must contain two key-value pairs: **name** is restricted to the [AC Name](#ac-name-type) formatting and **value** is an arbitrary string). Annotation names must be unique within the list.
* **ports** (list of objects, optional) list of ports that will be exposed on the host.
    * **name** (string, required) name of the port in the image manifest that will be exposed on the host (restricted to the [AC Name](#ac-name-type) formatting).
    * **hostPort** (integer, required) port number on the host that will be mapped to the container port.
