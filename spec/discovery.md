## App Container Image Discovery

An App Container Image name has a URL-like structure, for example `example.com/reduce-worker`.
However, there is no scheme on this name, so it cannot be directly resolved to an App Container Image URL.
Furthermore, attributes other than the name may be required to unambiguously identify an image (version, OS and architecture).
These attributes are expressed in the **labels** field of the [Image Manifest](aci.md#image-manifest-schema).
App Container Image Discovery prescribes a discovery process to retrieve an image based on the name and these attributes.

Image Discovery is inspired by Go's [remote import paths](https://golang.org/cmd/go/#hdr-Remote_import_paths).

There are three URLs types:

* Image URLs
* Signature URLs
* Public key URLs
* Torrent URLs

The Torrent URLs are torrent files for download image,signature(optional) and public key(optional) by p2p(peer to peer).Client should parse torrent file and start download image by p2p after torrent file is downloaded.

Simple and Meta Discovery processes use one or more templates (predefined or derived from various sources) to render Image and Signature URLs (while the Public keys URLs aren't templates).

Note that, to discriminate between the image and its signature, the templates must contain `{ext}` and its values MUST be `aci` (for the image), `aci.asc` (for the signature) or `torrent`(for the torrent file). 

### 1.Simple Discovery

The simple discovery template is:

    https://{name}-{version}-{os}-{arch}.{ext}

####1.1 client-server

First, try to fetch the App Container Image by rendering the above template (with `{ext}` rendered to `aci`) and directly retrieving the resulting URL.

For example, given the image `{name}`: `example.com/reduce-worker`, with `{version}`: `1.0.0`, `{arch}`: `amd64`, and `{os}`: `linux`, try to retrieve:

    https://example.com/reduce-worker-1.0.0-linux-amd64.aci

If this fails, move on to meta discovery.
If this succeeds, try fetching the signature using the same template but with `{ext}` rendered to `aci.asc`:

    https://example.com/reduce-worker-1.0.0-linux-amd64.aci.asc
####1.2 p2p(peer to peer)
Try to fetch the torrent by rendering the above template (with `{ext}` rendered to `torrent`) and directly retrieving the resulting URL.



Simple discovery does not provide a way to discover Public Keys.

### 2.Meta Discovery

If simple discovery fails, then we use HTTPS+HTML `meta` tags retrieved from a "discovery URL" to resolve an image name to downloadable URLs.

The client-server download template for the discovery URL is:

    https://{name}?ac-discovery=1
	
The p2p download template for the discovery URL is:

    https://{name}?ac-discovery-p2p=1
	
For example, if the client is looking for `example.com/reduce-worker` it will request:

    https://example.com/reduce-worker?ac-discovery=1
	https://example.com/reduce-worker?ac-discovery-p2p=1


	

then inspect the HTML returned for `meta` tags that have the following format:

```html
<meta name="ac-discovery" content="prefix-match url-tmpl">
<meta name="ac-discovery-p2p" content="prefix-match url-tmpl">
<meta name="ac-discovery-pubkeys" content="prefix-match url">
```

* `ac-discovery` MUST contain a URL template that can be rendered to retrieve the ACI or associated signature
* `ac-discovery-p2p` MUST contain a URL template that can be rendered to retrieve the torrent file which use for download the ACI,signature and public key.
* `ac-discovery-pubkeys` SHOULD contain a URL that provides a set of public keys that can be used to verify the signature of the ACI

Some examples for different schemes and URLs:

```html
<meta name="ac-discovery" content="example.com https://storage.example.com/{os}/{arch}/{name}-{version}.{ext}">
<meta name="ac-discovery" content="example.com hdfs://storage.example.com/{name}-{version}-{os}-{arch}.{ext}">
<meta name="ac-discovery-p2p" content="example.com hdfs://storage.example.com/{name}-{version}-{os}-{arch}.{ext}">
<meta name="ac-discovery-pubkeys" content="example.com https://example.com/pubkeys.gpg">
```

When evaluating `ac-discovery` tags, the client MUST first ensure that the prefix of the [AC Name](types.md#ac-name-type) being discovered matches the prefix-match, and if so it MUST perform a simple template substitution to determine the URL at which the resource can be retrieved - the effective equivalent of:

```bash
urltmpl="https://{name}-{version}-{os}-{arch}.{ext}"
curl $(echo "$urltmpl" | sed -e "s/{name}/$name/" -e "s/{version}/$version/" -e "s/{os}/$os/" -e "s/{arch}/$arch/" -e "s/{ext}/$ext/")
```

If it's `ac-discovery` tag, where _name_, _version_, _os_, and _arch_ are set to their respective values for the image, and _ext_ is either `aci` or `aci.asc` for retrieving an App Container Image or signature respectively. 
If it's `ac-discovery-p2p` tag, where _name_, _version_, _os_, and _arch_ are set to their respective values for the torrent, and  _ext_ is `torrent` for retrieving a torrent file of App Container Image. 

Note that multiple `ac-discovery` tags MAY be returned for a given prefix-match (for example, with different scheme names representing different transport mechanisms).
In this case, the client implementation MAY choose which to use at its own discretion.
Public discovery implementations SHOULD always provide at least one HTTPS URL template.

In our example above, using the HTTPS URL template, the client would attempt to retrieve the following URLs:

```
Signature: 	https://storage.example.com/linux/amd64/reduce-worker-1.0.0.aci.asc
ACI: 		https://storage.example.com/linux/amd64/reduce-worker-1.0.0.aci
Keys: 		https://example.com/pubkeys.gpg
Torrent:	https://storage.example.com/linux/amd64/reduce-worker-1.0.0.torrent
```

If the first attempt at fetching the initial discovery URL returns a `4xx` status code or does not contain any `ac-discovery` meta tags then attempt the parent path in the `name`.
For example if the user has `example.com/project/subproject` and we first try `example.com/project/subproject` but do not discover a `meta` tag then try `example.com/project` and then try `example.com`.

All HTTP redirects MUST be followed when the discovery URL returns a `3xx` status code.

Discovery URLs that require interpolation are [RFC6570](https://tools.ietf.org/html/rfc6570) URI templates.

### Validation

Implementations of the spec are responsible for enforcing any signature validation rules set in place by the operator.
For example, in a testing environment, signature validation might be disabled, in which case the implementation would omit the signature retrieval.

Implementations MUST ensure that the initial name and labels used for discovery matches the name and labels in the Image Manifest.

A label is considered to match if it meets one of two criteria:
- It is present in the discovery labels and present in the Image Manifest with the same value.
- It is absent from the discovery labels and present in Image Manifest, with any value.

### Authentication

Authentication during the discovery process is optional.
If an attempt at fetching any resource (the initial discovery URL, an App Container Image, or signature) returns a `401 Unauthorized`, implementations should enact the authentication policy set by the operator.
For example, some implementations might only perform HTTP basic authentication over HTTPS connections.

