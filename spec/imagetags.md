## Image Tags

The [image tags](#image-tags-schema) is a [JSON](https://tools.ietf.org/html/rfc4627) file that includes information needed to resolve a tag to a set of labels.

### Image Tags Schema

JSON Schema for the Image Tags, conforming to [RFC4627](https://tools.ietf.org/html/rfc4627)

```json
{
    "aliases": {
        "latest": "3.x",
        "3.x": "3.0.x",
        "3.0.x": "3.0.1",
        "3.0.1": "3.0.1-2"
    },
    "labels": {
        "3.0.1-2" : { "version": "3.0.1", "build": "2" },
        "3.0.1-3" : { "version": "3.0.1", "build": "3" }
    }
}
```

* **aliases** (map, optional): map of tag aliases. The key is the tag name, the value it the tag alias.
* **labels** (map, optional): map of tag labels. The key is the tag name, the value is a map of labels where the key is the label name (string, restricted to the [AC Identifier](types.md#ac-identifier-type) formatting) and the value is the label value (string).


### Labels resolution

The labels resolution process uses image tags data to resolve a tag to a set of labels.

The resolution process is made of these phases:

1. Find an alias for the provided tag. If not found go to the next phase. If found use the alias as the new tag and repeat this phase.
2. Find a set of labels for the tag. If no labels can be found return an error.


### Labels Merging

The labels merging process merges the tags resolved labels with an initial set of label to obtain a final set of label that can be used for Image discovery
The labels merging process is made of these phases:

1. If no tag map is provided then set the _version_ label to the tag value and exit. If the initial labels already have a _version_ label return an error.
2. If no tag is provided just keep the initial labels and exit.
3. Merge the tag resolved labels with the initial labels only if the label is not provided by the initial labels (initial label takes the precedence).



