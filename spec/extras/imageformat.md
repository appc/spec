## Image format string

This specification defines an image format string to use to reference an image. It can be used by discovery or by an ACI to let the user define the image to discovery, execute etc...

An image format string consists of an image name and a set of labels with this format:

```
name[:versionvalue][,labelname=labelvalue][,labelname=labelvalue]...
```

Where:
* name (string, required) a human-readable name for an App Container Image (string, restricted to the AC Identifier formatting)
* versionvalue (string, optional) is a shortcut to define a version label. Writing `name:versionvalue` is the same of writing `name,version=versionvalue`
* labelname=labelvalue (optional) A label definition, must have two key-value pairs: *labelname* is restricted to the AC Identifier formatting and *labelvalue* is an arbitrary string.

Since *labelvalue* (and *versionvalue*) can be an arbitrary string it MUST be URL escaped (percent encoded) (http://tools.ietf.org/html/rfc3986#section-2) with UTF-8 encoding.


### Examples

An image format string for an image with name "example.com/reduce-worker" with version "0.1.0+gitabcdef" will be:

`example.com/reduce-worker:0.1.0%2Bgitabcdef` or
`example.com/reduce-worker,version=0.1.0%2Bgitabcdef`

Note that the `+` in the *versionvalue* has been replace by `%2B` since the label value MUST be URL encoded.
