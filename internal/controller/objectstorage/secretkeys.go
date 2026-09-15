package objectstorage

// Canonical data keys for the Secrets the object storage adapters write.
// Shared by the ObjectStorageAccessKey adapter and its tests so the S3
// credentials Secret uses one spelling.
const (
	SecretKeyAccessKeyID     = "AWS_ACCESS_KEY_ID"
	SecretKeySecretAccessKey = "AWS_SECRET_ACCESS_KEY"
	SecretKeyEndpointURL     = "AWS_ENDPOINT_URL"
	SecretKeyRegion          = "AWS_REGION"
)

// EndpointTypePublic is the type of the public S3 endpoint of a service.
const EndpointTypePublic = "public"
