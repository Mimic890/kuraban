package handlers

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"Mimic890/kuraban/internal/models"

	"github.com/gofiber/fiber/v3"
)

// maxUploadFiles caps how many files a single multipart upload-multiple
// request may contain. Without a limit, a request with an unbounded number
// of form parts opens one file descriptor per part before any upload work
// happens, letting a client exhaust file descriptors/memory (DoS).
const maxUploadFiles = 200

// unsafeInlineContentTypes are MIME types that a browser can execute as
// JavaScript in the response's origin when rendered inline. Since the SPA is
// served from the same origin as the API, any uploader could otherwise plant
// stored XSS by uploading a file with one of these Content-Types.
var unsafeInlineContentTypes = map[string]struct{}{
	"text/html":              {},
	"application/xhtml+xml":  {},
	"image/svg+xml":          {},
	"application/xml":        {},
	"text/xml":               {},
	"application/javascript": {},
	"text/javascript":        {},
}

// safeContentType rewrites Content-Types that the browser would treat as
// executable to application/octet-stream.
func safeContentType(ct string) string {
	base := strings.TrimSpace(strings.ToLower(ct))
	if i := strings.IndexByte(base, ';'); i >= 0 {
		base = strings.TrimSpace(base[:i])
	}
	if _, bad := unsafeInlineContentTypes[base]; bad {
		return "application/octet-stream"
	}
	return ct
}

// contentDispositionHeader builds an RFC 6266 / RFC 5987 Content-Disposition
// header value with the user-controlled object key safely encoded. Strips
// path components and control characters before emitting the ASCII fallback,
// then appends the percent-encoded UTF-8 filename*= for full fidelity.
func contentDispositionHeader(disposition, key string) string {
	name := path.Base(key)
	if name == "." || name == "/" || name == "" {
		name = "download"
	}
	// ASCII-safe fallback: drop anything that could break the quoted value.
	var asciiFallback strings.Builder
	for _, r := range name {
		if r < 0x20 || r == 0x7f || r == '"' || r == '\\' || r > 0x7e {
			asciiFallback.WriteByte('_')
			continue
		}
		asciiFallback.WriteRune(r)
	}
	fallback := asciiFallback.String()
	if fallback == "" {
		fallback = "download"
	}
	encoded := url.PathEscape(name)
	return disposition + "; filename=\"" + fallback + "\"; filename*=UTF-8''" + encoded
}

// PreviewTokenMinter mints signed single-object preview tokens.
// auth.Service satisfies it.
type PreviewTokenMinter interface {
	MintPreviewToken(bucket, key string, ttl time.Duration) (string, time.Time, error)
}

// previewTokenTTL is long enough that seeking mid-playback keeps working.
// The frontend mints a fresh URL when a token expires.
const previewTokenTTL = 15 * time.Minute

// ObjectHandler handles object-related HTTP requests.
type ObjectHandler struct {
	previewTokens PreviewTokenMinter
}

type conditionalRangeGetter interface {
	GetObjectRangeIfMatch(ctx context.Context, bucketName, key string, start, end int64, etag string) (io.ReadCloser, error)
}

// NewObjectHandler creates a new object handler.
func NewObjectHandler(previewTokens PreviewTokenMinter) *ObjectHandler {
	return &ObjectHandler{
		previewTokens: previewTokens,
	}
}

// ListObjects lists objects in a bucket with optional filtering and pagination
//
//	@Summary		List objects in a bucket
//	@Description	Retrieves a list of objects and prefixes (folders) stored in the specified bucket, with optional filtering by prefix, pagination support, and max keys
//	@Tags			Objects
//	@Accept			json
//	@Produce		json
//	@Param			bucket				path		string												true	"Name of the bucket to list objects from"
//	@Param			prefix				query		string												false	"Filter objects by prefix"
//	@Param			search				query		string												false	"Recursively search object keys under prefix by case-insensitive substring (best-effort; bypasses max_keys and continuation_token)"
//	@Param			max_keys			query		int													false	"Maximum number of objects to return (default: 100)"
//	@Param			continuation_token	query		string												false	"Token for pagination to retrieve next page of results"
//	@Success		200					{object}	models.APIResponse{data=models.ObjectListResponse}	"Successfully retrieved list of objects and prefixes"
//	@Failure		400					{object}	models.APIResponse{error=models.APIError}			"Invalid request parameters"
//	@Failure		404					{object}	models.APIResponse{error=models.APIError}			"Bucket not found"
//	@Failure		500					{object}	models.APIResponse{error=models.APIError}			"Failed to list objects"
//	@Router			/api/v1/buckets/{bucket}/objects [get]
func (h *ObjectHandler) ListObjects(c fiber.Ctx) error {
	ctx := c.Context()

	// Get bucket name from URL parameter
	bucketName := c.Params("bucket")
	if bucketName == "" {
		return c.Status(fiber.StatusBadRequest).JSON(
			models.ErrorResponse(models.ErrCodeBadRequest, "Bucket name is required"),
		)
	}

	// Get query parameters for filtering and pagination
	prefix := c.Query("prefix", "")

	// Search mode: a recursive, best-effort substring search across the whole
	// subtree under prefix. S3/Garage has no server-side substring search, so
	// the backend scans and filters. This bypasses page-token pagination and
	// max_keys, see S3Service.SearchObjects -> pagination is handled frontend side.
	if search := c.Query("search", ""); search != "" {
		results, err := getS3Service(c).SearchObjects(ctx, bucketName, prefix, search)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(
				models.ErrorResponse(models.ErrCodeListFailed, "Failed to search objects: "+err.Error()),
			)
		}
		return c.JSON(models.SuccessResponse(results))
	}

	continuationToken := c.Query("continuation_token", "")

	maxKeysStr := c.Query("max_keys", "100")
	maxKeys, err := strconv.Atoi(maxKeysStr)
	if err != nil || maxKeys <= 0 {
		return c.Status(fiber.StatusBadRequest).JSON(
			models.ErrorResponse(models.ErrCodeBadRequest, "max_keys must be a positive integer"),
		)
	}
	if maxKeys > 1000 {
		maxKeys = 1000
	}

	// List objects in the bucket
	objects, err := getS3Service(c).ListObjects(ctx, bucketName, prefix, maxKeys, continuationToken)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(
			models.ErrorResponse(models.ErrCodeListFailed, "Failed to list objects: "+err.Error()),
		)
	}

	return c.JSON(models.SuccessResponse(objects))
}

// UploadObject uploads an object to a bucket
//
//	@Summary		Upload object to bucket
//	@Description	Uploads an object to the specified bucket using multipart/form-data
//	@Tags			Objects
//	@Accept			multipart/form-data
//	@Produce		json
//	@Param			bucket	path		string													true	"Name of the bucket to upload the object to"
//	@Param			file	formData	file													true	"File to upload"
//	@Param			key		formData	string													false	"Object key (path in bucket). If not provided, the filename will be used"
//	@Success		201		{object}	models.APIResponse{data=models.ObjectUploadResponse}	"Object uploaded successfully"
//	@Failure		400		{object}	models.APIResponse{error=models.APIError}				"Invalid request parameters"
//	@Failure		404		{object}	models.APIResponse{error=models.APIError}				"Bucket not found"
//	@Failure		500		{object}	models.APIResponse{error=models.APIError}				"Failed to upload object"
//	@Router			/api/v1/buckets/{bucket}/objects [post]
func (h *ObjectHandler) UploadObject(c fiber.Ctx) error {
	ctx := c.Context()

	// Get bucket name from URL parameter
	bucketName := c.Params("bucket")
	if bucketName == "" {
		return c.Status(fiber.StatusBadRequest).JSON(
			models.ErrorResponse(models.ErrCodeBadRequest, "Bucket name is required"),
		)
	}

	// Get file from multipart form
	file, err := c.FormFile("file")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(
			models.ErrorResponse(models.ErrCodeBadRequest, "File is required: "+err.Error()),
		)
	}

	// Get object key (path in bucket)
	key := c.FormValue("key")
	if key == "" {
		// Use filename as key if not provided
		key = file.Filename
	}

	// Open the uploaded file
	fileHandle, err := file.Open()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(
			models.ErrorResponse(models.ErrCodeUploadFailed, "Failed to open uploaded file: "+err.Error()),
		)
	}
	defer fileHandle.Close()

	// Get content type
	contentType := file.Header.Get("Content-Type")

	// Upload to Garage
	uploadResult, err := getS3Service(c).UploadObject(ctx, bucketName, key, fileHandle, file.Size, contentType)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(
			models.ErrorResponse(models.ErrCodeUploadFailed, "Failed to upload object: "+err.Error()),
		)
	}

	return c.Status(fiber.StatusCreated).JSON(models.SuccessResponse(uploadResult))
}

// CreateDirectory creates an empty directory marker in a bucket.
//
//	@Summary		Create directory in bucket
//	@Description	Creates a zero-byte object whose key ends with "/" so that S3 clients display it as an empty folder.
//	@Tags			Objects
//	@Accept			json
//	@Produce		json
//	@Param			bucket	path		string												true	"Name of the bucket"
//	@Param			request	body		object{key=string}									true	"Directory key (must end with '/')"
//	@Success		201		{object}	models.APIResponse{data=models.ObjectUploadResponse}	"Directory created"
//	@Failure		400		{object}	models.APIResponse{error=models.APIError}			"Invalid request parameters"
//	@Failure		500		{object}	models.APIResponse{error=models.APIError}			"Failed to create directory"
//	@Router			/api/v1/buckets/{bucket}/directories [post]
func (h *ObjectHandler) CreateDirectory(c fiber.Ctx) error {
	ctx := c.Context()

	bucketName := c.Params("bucket")
	if bucketName == "" {
		return c.Status(fiber.StatusBadRequest).JSON(
			models.ErrorResponse(models.ErrCodeBadRequest, "Bucket name is required"),
		)
	}

	var req struct {
		Key string `json:"key"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(
			models.ErrorResponse(models.ErrCodeBadRequest, "Invalid request body: "+err.Error()),
		)
	}

	key := strings.TrimLeft(req.Key, "/")
	if key == "" {
		return c.Status(fiber.StatusBadRequest).JSON(
			models.ErrorResponse(models.ErrCodeBadRequest, "Directory key is required"),
		)
	}
	if !strings.HasSuffix(key, "/") {
		key += "/"
	}

	result, err := getS3Service(c).CreateDirectoryMarker(ctx, bucketName, key)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(
			models.ErrorResponse(models.ErrCodeUploadFailed, "Failed to create directory: "+err.Error()),
		)
	}

	return c.Status(fiber.StatusCreated).JSON(models.SuccessResponse(result))
}

// GetObject retrieves an object from a bucket
//
//	@Summary		Get object from bucket
//	@Description	Retrieves an object stored in the specified bucket
//	@Tags			Objects
//	@Accept			json
//	@Produce		application/octet-stream
//	@Param			bucket		path		string										true	"Name of the bucket containing the object"
//	@Param			key			path		string										true	"Key (path) of the object"
//	@Param			download	query		bool										false	"Set to true to download the object as an attachment"
//	@Success		200			{file}		binary										"Successfully retrieved the object"
//	@Failure		400			{object}	models.APIResponse{error=models.APIError}	"Bucket name and object key are required"
//	@Failure		404			{object}	models.APIResponse{error=models.APIError}	"Object not found"
//	@Router			/api/v1/buckets/{bucket}/objects/{key} [get]
func (h *ObjectHandler) GetObject(c fiber.Ctx) error {
	bucketName := c.Params("bucket")

	key, ok := c.Locals("objectKey").(string)
	if !ok || key == "" {
		key = c.Params("key")
	}

	if bucketName == "" || key == "" {
		return c.Status(fiber.StatusBadRequest).JSON(
			models.ErrorResponse(models.ErrCodeBadRequest, "Bucket name and object key are required"),
		)
	}

	// Range requests stream a partial body so media elements can seek.
	if rangeHeader := c.Get("Range"); rangeHeader != "" {
		return h.getObjectRange(c, bucketName, key, rangeHeader)
	}
	return h.serveFullObject(c, bucketName, key)
}

// serveFullObject streams the whole object with a 200, the pre-Range behavior.
func (h *ObjectHandler) serveFullObject(c fiber.Ctx, bucketName, key string) error {
	ctx := context.WithoutCancel(c.Context())

	body, objectInfo, err := getS3Service(c).GetObject(ctx, bucketName, key)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(
			models.ErrorResponse(models.ErrCodeObjectNotFound, "Object not found: "+err.Error()),
		)
	}

	// The uploader controls Content-Type. Rewrite executable MIME types to
	// application/octet-stream and always disable sniffing so stored HTML/SVG
	// cannot run as XSS in the SPA origin when fetched inline.
	c.Set("Content-Type", safeContentType(objectInfo.ContentType))
	c.Set("X-Content-Type-Options", "nosniff")
	c.Set("Accept-Ranges", "bytes")
	c.Set("Content-Length", strconv.FormatInt(objectInfo.Size, 10))
	c.Set("ETag", objectInfo.ETag)
	c.Set("Last-Modified", objectInfo.LastModified.Format(time.RFC1123))

	// The object key is attacker-controlled. Build the header via the safe
	// RFC 6266 helper to avoid quote/semicolon injection into filename=.
	disposition := "inline"
	if c.Query("download") == "true" {
		disposition = "attachment"
	}
	c.Set("Content-Disposition", contentDispositionHeader(disposition, key))
	c.Response().SetBodyStream(body, int(objectInfo.Size))
	return nil
}

// getObjectRange serves a single-range request with 206 Partial Content.
// Malformed and multi-range headers fall back to the full 200 response.
func (h *ObjectHandler) getObjectRange(c fiber.Ctx, bucketName, key, rangeHeader string) error {
	ctx := context.WithoutCancel(c.Context())

	info, err := getS3Service(c).GetObjectMetadata(ctx, bucketName, key)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(
			models.ErrorResponse(models.ErrCodeObjectNotFound, "Object not found: "+err.Error()),
		)
	}

	rng, unsatisfiable := parseRangeHeader(rangeHeader, info.Size)
	if unsatisfiable {
		c.Set("Accept-Ranges", "bytes")
		c.Set("Content-Range", "bytes */"+strconv.FormatInt(info.Size, 10))
		return c.SendStatus(fiber.StatusRequestedRangeNotSatisfiable)
	}
	if rng == nil {
		return h.serveFullObject(c, bucketName, key)
	}

	storage := getS3Service(c)
	var body io.ReadCloser
	if conditional, ok := storage.(conditionalRangeGetter); ok {
		body, err = conditional.GetObjectRangeIfMatch(ctx, bucketName, key, rng.start, rng.end, info.ETag)
	} else {
		body, err = storage.GetObjectRange(ctx, bucketName, key, rng.start, rng.end)
	}
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(
			models.ErrorResponse(models.ErrCodeObjectNotFound, "Object not found: "+err.Error()),
		)
	}

	c.Set("Content-Type", safeContentType(info.ContentType))
	c.Set("X-Content-Type-Options", "nosniff")
	c.Set("Accept-Ranges", "bytes")
	c.Set("Content-Length", strconv.FormatInt(rng.end-rng.start+1, 10))
	c.Set("Content-Range", "bytes "+strconv.FormatInt(rng.start, 10)+"-"+strconv.FormatInt(rng.end, 10)+"/"+strconv.FormatInt(info.Size, 10))
	c.Set("ETag", info.ETag)
	c.Set("Last-Modified", info.LastModified.Format(time.RFC1123))

	disposition := "inline"
	if c.Query("download") == "true" {
		disposition = "attachment"
	}
	c.Set("Content-Disposition", contentDispositionHeader(disposition, key))
	c.Status(fiber.StatusPartialContent)
	c.Response().SetBodyStream(body, int(rng.end-rng.start+1))
	return nil
}

// DeleteObject deletes an object from a bucket
//
//	@Summary		Delete object from bucket
//	@Description	Deletes an object stored in the specified bucket
//	@Tags			Objects
//	@Accept			json
//	@Produce		json
//	@Param			bucket	path		string													true	"Name of the bucket containing the object"
//	@Param			key		path		string													true	"Key (path) of the object"
//	@Success		200		{object}	models.APIResponse{data=models.ObjectDeleteResponse}	"Successfully deleted the object"
//	@Failure		400		{object}	models.APIResponse{error=models.APIError}				"Bucket name and object key are required"
//	@Failure		404		{object}	models.APIResponse{error=models.APIError}				"Object not found"
//	@Failure		500		{object}	models.APIResponse{error=models.APIError}				"Failed to delete object"
//	@Router			/api/v1/buckets/{bucket}/objects/{key} [delete]
func (h *ObjectHandler) DeleteObject(c fiber.Ctx) error {
	ctx := c.Context()

	// Get bucket name from URL parameters
	bucketName := c.Params("bucket")

	// Get object key from locals (set by route handler) or from params
	key, ok := c.Locals("objectKey").(string)
	if !ok || key == "" {
		key = c.Params("key")
	}

	if bucketName == "" || key == "" {
		return c.Status(fiber.StatusBadRequest).JSON(
			models.ErrorResponse(models.ErrCodeBadRequest, "Bucket name and object key are required"),
		)
	}

	// S3 delete is idempotent; checking first creates a read-before-delete race.
	if err := getS3Service(c).DeleteObject(ctx, bucketName, key); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(
			models.ErrorResponse(models.ErrCodeDeleteFailed, "Failed to delete object: "+err.Error()),
		)
	}

	// Return success response
	response := models.ObjectDeleteResponse{
		Bucket:  bucketName,
		Key:     key,
		Deleted: true,
	}

	return c.JSON(models.SuccessResponse(response))
}

// GetObjectMetadata returns metadata for an object without downloading it
//
//	@Summary		Get object metadata
//	@Description	Retrieves metadata information about an object without downloading the actual content
//	@Tags			Objects
//	@Accept			json
//	@Produce		json
//	@Param			bucket	path		string										true	"Name of the bucket containing the object"
//	@Param			key		path		string										true	"Key (path) of the object"
//	@Success		200		{object}	models.APIResponse{data=models.ObjectInfo}	"Successfully retrieved object metadata"
//	@Failure		400		{object}	models.APIResponse{error=models.APIError}	"Bucket name and object key are required"
//	@Failure		404		{object}	models.APIResponse{error=models.APIError}	"Object not found"
//	@Router			/api/v1/buckets/{bucket}/objects/{key}/metadata [get]
func (h *ObjectHandler) GetObjectMetadata(c fiber.Ctx) error {
	ctx := c.Context()

	// Get bucket name from URL parameters
	bucketName := c.Params("bucket")

	// Get object key from locals (set by route handler) or from params
	key, ok := c.Locals("objectKey").(string)
	if !ok || key == "" {
		key = c.Params("key")
	}

	if bucketName == "" || key == "" {
		return c.Status(fiber.StatusBadRequest).JSON(
			models.ErrorResponse(models.ErrCodeBadRequest, "Bucket name and object key are required"),
		)
	}

	// Get object metadata
	metadata, err := getS3Service(c).GetObjectMetadata(ctx, bucketName, key)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(
			models.ErrorResponse(models.ErrCodeObjectNotFound, "Object not found: "+err.Error()),
		)
	}

	c.Set("Accept-Ranges", "bytes")
	return c.JSON(models.SuccessResponse(metadata))
}

// GetPresignedURL generates a pre-signed URL for accessing an object
//
//	@Summary		Get pre-signed URL for object
//	@Description	Generates a pre-signed URL that allows temporary access to the specified object
//	@Tags			Objects
//	@Accept			json
//	@Produce		json
//	@Param			bucket		path		string													true	"Name of the bucket containing the object"
//	@Param			key			path		string													true	"Key (path) of the object"
//	@Param			expires_in	query		int														false	"Expiration time in seconds for the pre-signed URL (default: 3600 seconds)"
//	@Success		200			{object}	models.APIResponse{data=models.PresignedURLResponse}	"Successfully generated pre-signed URL"
//	@Failure		400			{object}	models.APIResponse{error=models.APIError}				"Invalid request parameters"
//	@Failure		404			{object}	models.APIResponse{error=models.APIError}				"Object not found"
//	@Failure		500			{object}	models.APIResponse{error=models.APIError}				"Failed to generate pre-signed URL"
//	@Router			/api/v1/buckets/{bucket}/objects/{key}/presign [get]
func (h *ObjectHandler) GetPresignedURL(c fiber.Ctx) error {
	ctx := c.Context()

	// Get bucket name from URL parameters
	bucketName := c.Params("bucket")

	// Get object key from locals (set by route handler) or from params
	key, ok := c.Locals("objectKey").(string)
	if !ok || key == "" {
		key = c.Params("key")
	}

	if bucketName == "" || key == "" {
		return c.Status(fiber.StatusBadRequest).JSON(
			models.ErrorResponse(models.ErrCodeBadRequest, "Bucket name and object key are required"),
		)
	}

	// Get expiration time from query parameter (default: 1 hour)
	expiresInStr := c.Query("expires_in", "3600")
	expiresIn, err := strconv.ParseInt(expiresInStr, 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(
			models.ErrorResponse(models.ErrCodeBadRequest, "Invalid expiration time: "+err.Error()),
		)
	}

	// Validate expiration time (1 second to 7 days)
	if expiresIn <= 0 || expiresIn > 604800 { // Max 7 days
		return c.Status(fiber.StatusBadRequest).JSON(
			models.ErrorResponse(models.ErrCodeBadRequest, "Invalid expiration time (must be between 1 and 604800 seconds)"),
		)
	}

	// Check if object exists
	exists, err := getS3Service(c).ObjectExists(ctx, bucketName, key)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(
			models.ErrorResponse(models.ErrCodeInternalError, "Failed to check object existence: "+err.Error()),
		)
	}

	if !exists {
		return c.Status(fiber.StatusNotFound).JSON(
			models.ErrorResponse(models.ErrCodeObjectNotFound, "Object not found"),
		)
	}

	// Generate pre-signed URL
	url, err := getS3Service(c).GetPresignedURL(ctx, bucketName, key, time.Duration(expiresIn)*time.Second)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(
			models.ErrorResponse(models.ErrCodeInternalError, "Failed to generate pre-signed URL: "+err.Error()),
		)
	}

	response := models.PresignedURLResponse{
		URL:       url,
		ExpiresIn: expiresIn,
		Bucket:    bucketName,
		Key:       key,
	}

	return c.JSON(models.SuccessResponse(response))
}

// GetPreviewURL mints a short-lived tokenized URL for streaming this object
//
//	@Summary		Get a tokenized preview URL for an object
//	@Description	Returns a relative URL carrying a short-lived token that authorizes streaming this object. Media elements cannot send an Authorization header, so the token rides in the URL instead.
//	@Tags			Objects
//	@Produce		json
//	@Param			bucket	path		string												true	"Name of the bucket containing the object"
//	@Param			key		path		string												true	"Key (path) of the object"
//	@Success		200		{object}	models.APIResponse{data=models.PreviewURLResponse}	"Preview URL minted"
//	@Failure		400		{object}	models.APIResponse{error=models.APIError}			"Bucket name and object key are required"
//	@Failure		500		{object}	models.APIResponse{error=models.APIError}			"Failed to mint the preview token"
//	@Router			/api/v1/buckets/{bucket}/objects/{key}/preview-url [get]
func (h *ObjectHandler) GetPreviewURL(c fiber.Ctx) error {
	bucketName := c.Params("bucket")

	key, ok := c.Locals("objectKey").(string)
	if !ok || key == "" {
		key = c.Params("key")
	}

	if bucketName == "" || key == "" {
		return c.Status(fiber.StatusBadRequest).JSON(
			models.ErrorResponse(models.ErrCodeBadRequest, "Bucket name and object key are required"),
		)
	}

	token, expiresAt, err := h.previewTokens.MintPreviewToken(bucketName, key, previewTokenTTL)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(
			models.ErrorResponse(models.ErrCodeInternalError, "Failed to mint the preview token: "+err.Error()),
		)
	}

	previewURL := "/api/v1/buckets/" + url.PathEscape(bucketName) +
		"/object?key=" + url.QueryEscape(key) + "&pt=" + url.QueryEscape(token)

	return c.JSON(models.SuccessResponse(models.PreviewURLResponse{
		URL:       previewURL,
		ExpiresAt: expiresAt.UTC().Format(time.RFC3339),
	}))
}

// DeleteMultipleObjects deletes multiple objects from a bucket
//
//	@Summary		Delete multiple objects from bucket
//	@Description	Deletes multiple objects stored in the specified bucket
//	@Tags			Objects
//	@Accept			json
//	@Produce		json
//	@Param			bucket	path		string																true	"Name of the bucket containing the objects"
//	@Param			request	body		object{keys=[]string,prefixes=[]string}								true	"Object keys to delete and/or folder prefixes to delete recursively"
//	@Success		200		{object}	models.APIResponse{data=models.ObjectDeleteMultipleResponse}	"Successfully deleted the objects"
//	@Failure		400		{object}	models.APIResponse{error=models.APIError}						"Invalid request parameters"
//	@Failure		404		{object}	models.APIResponse{error=models.APIError}						"Bucket not found"
//	@Failure		500		{object}	models.APIResponse{error=models.APIError}						"Failed to delete objects"
//	@Router			/api/v1/buckets/{bucket}/objects/delete-multiple [post]
func (h *ObjectHandler) DeleteMultipleObjects(c fiber.Ctx) error {
	ctx := c.Context()

	// Get bucket name from URL parameter
	bucketName := c.Params("bucket")
	if bucketName == "" {
		return c.Status(fiber.StatusBadRequest).JSON(
			models.ErrorResponse(models.ErrCodeBadRequest, "Bucket name is required"),
		)
	}

	// Parse request body. "keys" are concrete objects to delete; "prefixes" are
	// folders to delete recursively (every object stored under the prefix).
	var req struct {
		Keys     []string `json:"keys"`
		Prefixes []string `json:"prefixes,omitempty"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(
			models.ErrorResponse(models.ErrCodeBadRequest, "Invalid request body: "+err.Error()),
		)
	}

	if len(req.Keys) == 0 && len(req.Prefixes) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(
			models.ErrorResponse(models.ErrCodeBadRequest, "At least one key or prefix is required"),
		)
	}

	// Validate and normalize folder prefixes before running an irreversible
	// recursive delete on a public endpoint. A blank prefix would match the
	// entire bucket, and a prefix without a trailing slash (e.g. "photos/2024")
	// would also match sibling keys such as "photos/2024-old/...". Reject blanks
	// with a 4XX and force a trailing slash so a prefix only ever deletes the
	// objects inside its own folder.
	prefixes := make([]string, 0, len(req.Prefixes))
	for _, p := range req.Prefixes {
		trimmed := strings.TrimSpace(p)
		if trimmed == "" {
			return c.Status(fiber.StatusBadRequest).JSON(
				models.ErrorResponse(models.ErrCodeBadRequest, "Prefix must not be blank"),
			)
		}
		if !strings.HasSuffix(trimmed, "/") {
			trimmed += "/"
		}
		prefixes = append(prefixes, trimmed)
	}

	deleted := 0

	// Delete the individually selected objects in a single batch call.
	if len(req.Keys) > 0 {
		n, err := getS3Service(c).DeleteMultipleObjects(ctx, bucketName, req.Keys)
		deleted += n
		if err != nil {
			return c.Status(deleteFailureStatus(deleted)).JSON(partialDeleteResponse(bucketName, deleted, req.Keys, prefixes))
		}
	}

	// Recursively delete every object under each selected folder prefix.
	for _, prefix := range prefixes {
		n, err := getS3Service(c).DeleteObjectsByPrefix(ctx, bucketName, prefix)
		deleted += n
		if err != nil {
			return c.Status(deleteFailureStatus(deleted)).JSON(partialDeleteResponse(bucketName, deleted, req.Keys, prefixes))
		}
	}

	response := models.ObjectDeleteMultipleResponse{
		Bucket:   bucketName,
		Deleted:  deleted,
		Keys:     req.Keys,
		Prefixes: prefixes,
	}

	return c.JSON(models.SuccessResponse(response))
}

func deleteFailureStatus(deleted int) int {
	if deleted > 0 {
		return fiber.StatusMultiStatus
	}
	return fiber.StatusInternalServerError
}

func partialDeleteResponse(bucket string, deleted int, keys, prefixes []string) models.APIResponse {
	return models.APIResponse{
		Success: false,
		Data:    models.ObjectDeleteMultipleResponse{Bucket: bucket, Deleted: deleted, Keys: keys, Prefixes: prefixes},
		Error:   &models.APIError{Code: models.ErrCodeDeleteFailed, Message: "Delete partially completed; refresh before retrying"},
	}
}

// UploadMultipleObjects uploads multiple objects to a bucket
//
//	@Summary		Upload multiple objects to bucket
//	@Description	Uploads multiple objects to the specified bucket using multipart/form-data. Accepts up to maxUploadFiles files per request.
//	@Tags			Objects
//	@Accept			multipart/form-data
//	@Produce		json
//	@Param			bucket	path		string															true	"Name of the bucket to upload the objects to"
//	@Param			files	formData	file															true	"Files to upload (can be multiple)"
//	@Success		201		{object}	models.APIResponse{data=models.ObjectUploadMultipleResponse}	"Objects uploaded successfully (including partial failures)"
//	@Failure		400		{object}	models.APIResponse{error=models.APIError}						"Invalid request parameters"
//	@Failure		404		{object}	models.APIResponse{error=models.APIError}						"Bucket not found"
//	@Failure		500		{object}	models.APIResponse{error=models.APIError}						"Failed to upload objects"
//	@Router			/api/v1/buckets/{bucket}/objects/upload-multiple [post]
func (h *ObjectHandler) UploadMultipleObjects(c fiber.Ctx) error {
	ctx := c.Context()

	// Get bucket name from URL parameter
	bucketName := c.Params("bucket")
	if bucketName == "" {
		return c.Status(fiber.StatusBadRequest).JSON(
			models.ErrorResponse(models.ErrCodeBadRequest, "Bucket name is required"),
		)
	}

	// Parse multipart form to get all files
	form, err := c.MultipartForm()
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(
			models.ErrorResponse(models.ErrCodeBadRequest, "Failed to parse multipart form: "+err.Error()),
		)
	}

	files := form.File["files"]
	if len(files) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(
			models.ErrorResponse(models.ErrCodeBadRequest, "At least one file is required"),
		)
	}
	if len(files) > maxUploadFiles {
		return c.Status(fiber.StatusBadRequest).JSON(
			models.ErrorResponse(models.ErrCodeBadRequest, fmt.Sprintf("Too many files: %d exceeds the limit of %d per request", len(files), maxUploadFiles)),
		)
	}

	// Prepare upload data structure
	uploadFiles := make([]struct {
		Key         string
		Body        io.Reader
		Size        int64
		ContentType string
	}, len(files))

	// Open all files and prepare for upload
	opened := make([]io.Closer, 0, len(files))
	keys := make(map[string]struct{}, len(files))
	for i, fileHeader := range files {
		file, err := fileHeader.Open()
		if err != nil {
			for _, f := range opened {
				f.Close()
			}
			return c.Status(fiber.StatusInternalServerError).JSON(
				models.ErrorResponse(models.ErrCodeUploadFailed, "Failed to open file "+fileHeader.Filename+": "+err.Error()),
			)
		}
		opened = append(opened, file)

		// Use filename as the key
		key := fileHeader.Filename
		if _, exists := keys[key]; exists {
			for _, f := range opened {
				_ = f.Close()
			}
			return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse(models.ErrCodeBadRequest, "duplicate upload key: "+key))
		}
		keys[key] = struct{}{}
		contentType := fileHeader.Header.Get("Content-Type")
		if contentType == "" {
			contentType = "application/octet-stream"
		}

		uploadFiles[i] = struct {
			Key         string
			Body        io.Reader
			Size        int64
			ContentType string
		}{
			Key:         key,
			Body:        file,
			Size:        fileHeader.Size,
			ContentType: contentType,
		}
	}

	// Upload all files using the service method
	results := getS3Service(c).UploadMultipleObjects(ctx, bucketName, uploadFiles)

	// Close all opened files immediately to free file descriptors
	for _, f := range opened {
		f.Close()
	}

	// Process results and categorize successes and failures
	var successFiles []models.ObjectUploadResult
	var failedFiles []models.ObjectUploadFailedResult
	successCount := 0
	failureCount := 0

	for _, result := range results {
		if result.Success {
			successCount++
			successFiles = append(successFiles, models.ObjectUploadResult{
				Key:         result.Key,
				ETag:        result.ETag,
				Size:        result.Size,
				ContentType: result.ContentType,
			})
		} else {
			failureCount++
			failedFiles = append(failedFiles, models.ObjectUploadFailedResult{
				Key:         result.Key,
				Error:       uploadErrorText(result.Error),
				ContentType: result.ContentType,
			})
		}
	}

	response := models.ObjectUploadMultipleResponse{
		Bucket:       bucketName,
		TotalFiles:   len(files),
		SuccessCount: successCount,
		FailureCount: failureCount,
		SuccessFiles: successFiles,
		FailedFiles:  failedFiles,
	}

	// Return 201 if all succeeded, 207 (Multi-Status) if partial success, 500 if all failed
	statusCode := fiber.StatusCreated
	if failureCount > 0 && successCount > 0 {
		statusCode = fiber.StatusMultiStatus // 207
	} else if failureCount > 0 && successCount == 0 {
		statusCode = fiber.StatusInternalServerError
	}

	return c.Status(statusCode).JSON(models.SuccessResponse(response))
}

func uploadErrorText(err error) string {
	if err == nil {
		return "upload failed"
	}
	return err.Error()
}

// EmptyBucket deletes all objects in a bucket
//
//	@Summary		Delete all objects in a bucket
//	@Description	Removes every object stored in the specified bucket. Used to empty a bucket before deletion.
//	@Tags			Objects
//	@Accept			json
//	@Produce		json
//	@Param			bucket	path		string												true	"Name of the bucket to empty"
//	@Success		200		{object}	models.APIResponse{data=object{bucket=string,deleted=int}}	"All objects deleted"
//	@Failure		400		{object}	models.APIResponse{error=models.APIError}			"Bucket name is required"
//	@Failure		500		{object}	models.APIResponse{error=models.APIError}			"Failed to delete objects"
//	@Router			/api/v1/buckets/{bucket}/objects/empty [post]
func (h *ObjectHandler) EmptyBucket(c fiber.Ctx) error {
	ctx := c.Context()

	bucketName := c.Params("bucket")
	if bucketName == "" {
		return c.Status(fiber.StatusBadRequest).JSON(
			models.ErrorResponse(models.ErrCodeBadRequest, "Bucket name is required"),
		)
	}

	deleted, err := getS3Service(c).DeleteAllObjects(ctx, bucketName)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(
			models.ErrorResponse(models.ErrCodeDeleteFailed, "Failed to empty bucket: "+err.Error()),
		)
	}

	return c.JSON(models.SuccessResponse(map[string]interface{}{
		"bucket":  bucketName,
		"deleted": deleted,
	}))
}
