## App Container Pods (pods)

The deployable, executable unit in the App Container specification is the **pod**.
A **pod** is a list of apps that will be launched together inside a shared execution context.
The execution context can be defined as the conjunction of several Linux namespaces (or equivalents on other operating systems):

- PID namespace (apps within the pod can see and signal each other's processes)
- network namespace (apps within the pod have access to the same IP and port space)
- IPC namespace (apps within the pod can use SystemV IPC or POSIX message queues to communicate)
- UTS namespace (apps within the pod share a hostname)

The context MAY include shared volumes, which are defined at the pod level and must be made available in each app's filesystem.
The context MAY additionally consist of one or more [isolators](ace.md#isolators).

The definition of the **pod** - namely, the list of constituent apps, and any isolators that apply to the entire pod - is codified in a [Pod Manifest](#pod-manifest-schema).
Pod Manifests can serve the role of both _deployable template_ and _runtime manifest_: a template can be a candidate for a series of transformations before execution.
For example, a Pod Manifest might reference an app with a label requirement of `version=latest`, which another tool might subsequently resolve to a specific version.
Another example would be that volumes are "late-bound" by the executor; alternatively, an executor might add annotations.
Pod Manifests also provide the ability to override application execution parameters for their constituent ACIs (i.e. the `app` section of the respective Image Manifests).

A Pod Manifest must be fully resolved (_reified_) before execution.
Specifically, a Pod Manifest must have all `mountPoint`s satisfied by `volume`s, and must reference all applications deterministically (by image ID).
At runtime, the reified Pod Manifest is exposed to applications through the [Metadata Service](ace.md#app-container-metadata-service).

### Pod Manifest Schema

JSON Schema for the Pod Manifest, conforming to [RFC4627](https://tools.ietf.org/html/rfc4627)

```json
{
    "acVersion": "0.6.1",
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
                {"volume": "work", "path": "/mnt/foo"}
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
                {"volume": "work", "path": "/mnt/bar"}
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

* **acVersion** (string, required) represents the version of the schema specification [AC Version Type](types.md#ac-version-type)
* **acKind** (string, required) must be an [AC Kind](types.md#ac-kind-type) of value "PodManifest"
* **apps** (list of objects, required) list of apps that will execute inside of this pod. Each app object has the following set of key-value pairs:
    * **name** (string, required) name of the app (restricted to [AC Name](types.md#ac-name-type) formatting). This is used to identify an app within a pod, and hence MUST be unique within the list of apps. This may be different from the name of the referenced image (see below); in this way, a pod can have multiple apps using the same underlying image.
    * **image** (object, required) identifiers of the image providing this app
        * **id** (string of type [Image ID](types.md#image-id-type), required) content hash of the image that this app will execute inside of
        * **name** (string, optional) name of the image (restricted to [AC Identifier](types.md#ac-identifier-type) formatting)
        * **labels** (list of objects, optional) additional labels characterizing the image
    * **app** (object, optional) substitute for the app object of the referred image's ImageManifest. See [Image Manifest Schema](aci.md#image-manifest-schema) for what the app object contains.
    * **mounts** (list of objects, optional) list of mounts mapping an app mountPoint to a volume. Each mount has the following set of key-value pairs:
      * **volume** (string, required) name of the volume that will fulfill this mount (restricted to the [AC Name](types.md#ac-name-type) formatting)
      * **path** (string, required) path inside of the app filesystem to mount the volume; generally this will come from one of an apps mountPoint paths
    * **annotations** (list of objects, optional) arbitrary metadata appended to the app. The annotation objects must have a *name* key that has a value that is restricted to the [AC Name](types.md#ac-name-type) formatting and *value* key that is an arbitrary string). Annotation names must be unique within the list. These will be merged with annotations provided by the image manifest when queried via the metadata service; values in this list take precedence over those in the image manifest.
* **volumes** (list of objects, optional) list of volumes which will be mounted into each application's filesystem
    * **name** (string, required) descriptive label for the volume. (restricted to the [AC Name](types.md#ac-name-type) formatting)
    * **readOnly** (boolean, optional, defaults to "false" if unsupplied) whether or not the volume will be mounted read only.
    * **kind** (string, required) either:
        * **empty** - creates an empty directory on the host and bind mounts it into the container. All containers in the pod share the mount, and the lifetime of the volume is equal to the lifetime of the pod (i.e. the directory on the host machine is removed when the pod's filesystem is garbage collected)
        * **host** - fulfills a mount point with a bind mount from a **source** directory on the host.
    * **source** (string, required if **kind** is "host") absolute path on host to be bind mounted under a mount point in each app's chroot.
* **isolators** (list of objects of type [Isolator](types.md#isolator-type), optional) list of isolation steps that will apply to this pod.
* **annotations** (list of objects, optional) arbitrary metadata the executor will make available to applications via the metadata service. Objects must contain two key-value pairs: **name** is restricted to the [AC Name](types.md#ac-name-type) formatting and **value** is an arbitrary string). Annotation names must be unique within the list.
* **ports** (list of objects, optional) list of ports that SHOULD be exposed on the host.
    * **name** (string, required, restricted to the [AC Name](#ac-name-type) formatting) name of the port to be exposed on the host. This field is a key referencing by name ports specified in the Image Manifest(s) of the app(s) within this Pod Manifest; consequently, port names MUST be unique among apps within a pod.
    * **hostPort** (integer, required) port number on the host that will be mapped to the application port.
