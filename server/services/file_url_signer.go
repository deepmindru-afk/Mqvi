package services

// FileURLSigner signs file URLs before they reach the client.
// ISP interface — services depend on this, not the concrete Signer.
type FileURLSigner interface {
	// SignURL signs a file URL if it matches the file prefix.
	// Legacy /api/uploads/ URLs pass through unchanged.
	SignURL(fileURL string) string
	// SignURLPtr is like SignURL but for *string fields (avatar_url, icon_url, etc.).
	SignURLPtr(fileURL *string) *string
}

// FileDeleter removes a file from disk given its stored URL.
// ISP interface wrapping files.Locator delete methods.
type FileDeleter interface {
	// DeleteFromURL is fire-and-forget. Errors (including missing files) are swallowed.
	// Use this from request paths where a failed delete is acceptable.
	DeleteFromURL(storedURL string)
	// DeleteFromURLChecked returns the underlying os.Remove error so callers can
	// queue retries. Missing files (os.IsNotExist) and empty/legacy URLs return nil.
	DeleteFromURLChecked(storedURL string) error
}
