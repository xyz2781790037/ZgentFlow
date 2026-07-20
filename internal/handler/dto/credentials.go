package dto

// CredentialFieldMetadata exposes only whether a secret field has been
// configured. Secret values are never serialized in normal API responses.
type CredentialFieldMetadata struct {
	Configured bool `json:"configured"`
}

// CredentialsResponse is the shared response shape for model and web-search
// credential subresources.
type CredentialsResponse struct {
	Fields map[string]CredentialFieldMetadata `json:"fields"`
}
