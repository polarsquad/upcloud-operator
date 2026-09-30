// Package samples embeds sample manifests so tests reference them at compile time.
package samples

import _ "embed"

// ObjectStoragePolicy is objectstorage_v1alpha1_objectstoragepolicy.yaml.
//
//go:embed objectstorage_v1alpha1_objectstoragepolicy.yaml
var ObjectStoragePolicy []byte
