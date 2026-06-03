package util

import (
	"bytes"
	"context"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"

	"github.com/ferdian3456/virdanproject/internal/constant"
	"github.com/ferdian3456/virdanproject/internal/model"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/h2non/bimg"
)

var (
	ErrInvalidImage    = errors.New("invalid image parameters")
	ErrImageProcessing = errors.New("failed to process image")
)

var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)

// =============================================================================
// Resource Pools
// =============================================================================

var bufferPool = sync.Pool{
	New: func() interface{} {
		return new(bytes.Buffer)
	},
}

func getBuffer() *bytes.Buffer {
	buf := bufferPool.Get().(*bytes.Buffer)
	buf.Reset()
	return buf
}

func putBuffer(buf *bytes.Buffer) {
	bufferPool.Put(buf)
}

// =============================================================================
// Validator — The Ultimate Pre-allocated Stateful Validator
// This pattern uses embedded chains to achieve 0 allocation and maximum speed by
// reusing memory internal to the Validator.
// =============================================================================

// Validator is a stateful, high-performance container for fail-fast validation.
// It uses pre-allocated internal chains to achieve zero heap allocations and
// minimal latency during the validation process.
type Validator struct {
	err *model.BadRequestError
	s   StringChain
	i   IntChain
	f   Float64Chain
}

// NewValidator creates and initializes a new Validator instance.
// Example:
//
//	v := util.NewValidator()
func NewValidator() *Validator {
	v := &Validator{}
	v.s.v = v
	v.i.v = v
	v.f.v = v
	return v
}

// Reset clears the validator's error state, allowing it to be reused
// for a new set of validations.
func (v *Validator) Reset() {
	v.err = nil
}

// Err returns the first validation error encountered, or nil if all validations passed.
func (v *Validator) Err() *model.BadRequestError {
	return v.err
}

// Validate returns the first validation error as an error interface, or nil.
// Avoids the typed-nil interface trap when assigning to a `var err error`.
func (v *Validator) Validate() error {
	if v.err == nil {
		return nil
	}
	return v.err
}

func (v *Validator) setError(code, message, param string) {
	if v.err == nil {
		v.err = &model.BadRequestError{
			Code:    code,
			Message: message,
			Param:   param,
		}
	}
}

// -----------------------------------------------------------------------------
// Cold Path Helpers (Optimized for Inlining)
// -----------------------------------------------------------------------------

func (v *Validator) failRequired(field string) {
	v.setError(constant.ERR_VALIDATION_CODE, field+" is required", field)
}

func (v *Validator) failMinLen(field string, n int) {
	v.setError(constant.ERR_VALIDATION_CODE, field+" must be at least "+strconv.Itoa(n)+" characters", field)
}

func (v *Validator) failMaxLen(field string, n int) {
	v.setError(constant.ERR_VALIDATION_CODE, field+" must be at most "+strconv.Itoa(n)+" characters", field)
}

func (v *Validator) failExactLen(field string, n int) {
	v.setError(constant.ERR_VALIDATION_CODE, field+" must be exactly "+strconv.Itoa(n)+" characters", field)
}

func (v *Validator) failEqual(field, targetName string) {
	v.setError(constant.ERR_VALIDATION_CODE, field+" must be equal to "+targetName, field)
}

func (v *Validator) failNotEqual(field, targetName string) {
	v.setError(constant.ERR_VALIDATION_CODE, field+" must not be equal to "+targetName, field)
}

func (v *Validator) failMin(field string, n int) {
	v.setError(constant.ERR_VALIDATION_CODE, field+" must be at least "+strconv.Itoa(n), field)
}

func (v *Validator) failMax(field string, n int) {
	v.setError(constant.ERR_VALIDATION_CODE, field+" must be at most "+strconv.Itoa(n), field)
}

func (v *Validator) failPositive(field string) {
	v.setError(constant.ERR_VALIDATION_CODE, field+" must be positive", field)
}

func (v *Validator) failIntEqual(field string, target int) {
	v.setError(constant.ERR_VALIDATION_CODE, field+" must be exactly "+strconv.Itoa(target), field)
}

func (v *Validator) failIntNotEqual(field string, target int) {
	v.setError(constant.ERR_VALIDATION_CODE, field+" must not be "+strconv.Itoa(target), field)
}

func (v *Validator) failFloatMin(field string, n float64) {
	v.setError(constant.ERR_VALIDATION_CODE, field+" must be at least "+strconv.FormatFloat(n, 'f', -1, 64), field)
}

func (v *Validator) failFloatMax(field string, n float64) {
	v.setError(constant.ERR_VALIDATION_CODE, field+" must be at most "+strconv.FormatFloat(n, 'f', -1, 64), field)
}

// =============================================================================
// StringChain — Pre-allocated
// =============================================================================

type StringChain struct {
	v     *Validator
	field string
	value string
}

// UUID initiates a validation chain for a UUID string value.
// Shorthand for v.String(field, value).UUID().
func (v *Validator) UUID(field, value string) *StringChain {
	return v.String(field, value).UUID()
}

// String initiates a validation chain for a string value.
// Example:
//
//	v.String("email", req.Email).Required().MinLen(5)
func (v *Validator) String(field, value string) *StringChain {
	v.s.field = field
	v.s.value = value
	if v.s.v == nil { // Lazy init pointers if literal struct used
		v.s.v = v
	}
	return &v.s
}

// Required ensures that the string is not empty or composed only of whitespace.
func (c *StringChain) Required() *StringChain {
	if c.v.err != nil {
		return c
	}
	if isWhitespace(c.value) {
		c.v.failRequired(c.field)
	}
	return c
}

// MinLen ensures the string has at least n characters (Unicode aware).
func (c *StringChain) MinLen(n int) *StringChain {
	if c.v.err != nil {
		return c
	}
	if utf8.RuneCountInString(c.value) < n {
		c.v.failMinLen(c.field, n)
	}
	return c
}

// MaxLen ensures the string has at most n characters (Unicode aware).
func (c *StringChain) MaxLen(n int) *StringChain {
	if c.v.err != nil {
		return c
	}
	if utf8.RuneCountInString(c.value) > n {
		c.v.failMaxLen(c.field, n)
	}
	return c
}

// Len ensures the string has exactly n characters (Unicode aware).
func (c *StringChain) Len(n int) *StringChain {
	if c.v.err != nil {
		return c
	}
	if utf8.RuneCountInString(c.value) != n {
		c.v.failExactLen(c.field, n)
	}
	return c
}

// Email ensures the string value is a valid email address.
func (c *StringChain) Email() *StringChain {
	if c.v.err != nil {
		return c
	}
	if !emailRegex.MatchString(c.value) {
		c.v.setError(constant.ERR_VALIDATION_CODE, c.field+" must be a valid email address", c.field)
	}
	return c
}

// Equal ensures the string value is exactly equal to the target.
func (c *StringChain) Equal(target, targetName string) *StringChain {
	if c.v.err != nil {
		return c
	}
	if c.value != target {
		c.v.failEqual(c.field, targetName)
	}
	return c
}

// NotEqual ensures the string value is not equal to the target.
func (c *StringChain) NotEqual(target, targetName string) *StringChain {
	if c.v.err != nil {
		return c
	}
	if c.value == target {
		c.v.failNotEqual(c.field, targetName)
	}
	return c
}

// OneOf ensures the string value matches one of the provided options.
func (c *StringChain) OneOf(options ...string) *StringChain {
	if c.v.err != nil {
		return c
	}
	for _, opt := range options {
		if c.value == opt {
			return c
		}
	}
	c.v.setError(constant.ERR_VALIDATION_CODE, c.field+" must be one of: "+strings.Join(options, ", "), c.field)
	return c
}

// Custom allows for a custom validation function to be applied to the string.
func (c *StringChain) Custom(fn func(string) bool, message string) *StringChain {
	if c.v.err != nil {
		return c
	}
	if !fn(c.value) {
		c.v.setError(constant.ERR_VALIDATION_CODE, c.field+" "+message, c.field)
	}
	return c
}

// Nickname validates string matches nickname pattern: letters, digits, underscore, dash.
// Skip if already failed or value is empty (Required handles empty).
func (c *StringChain) Nickname() *StringChain {
	if c.v.err != nil || c.value == "" {
		return c
	}
	if !nicknameRegex.MatchString(c.value) {
		c.v.setError(constant.ERR_VALIDATION_CODE, c.field+" only allows letters, digits, underscore, dash", c.field)
	}
	return c
}

// Regex validates string against a pre-compiled regex. Escape hatch for one-off patterns.
// Caller MUST pass a pre-compiled *regexp.Regexp (compile-per-call is forbidden on the hot path).
func (c *StringChain) Regex(re *regexp.Regexp, message string) *StringChain {
	if c.v.err != nil || c.value == "" {
		return c
	}
	if !re.MatchString(c.value) {
		c.v.setError(constant.ERR_VALIDATION_CODE, message, c.field)
	}
	return c
}

// UUID validates string is a valid UUID format.
// Skip if value empty (caller should chain .Required() if mandatory).
func (c *StringChain) UUID() *StringChain {
	if c.v.err != nil || c.value == "" {
		return c
	}
	if _, err := uuid.Parse(c.value); err != nil {
		c.v.setError(constant.ERR_VALIDATION_CODE, c.field+" is not a valid UUID", c.field)
	}
	return c
}

func (c *StringChain) GetErr() *model.BadRequestError { return c.v.err }

// =============================================================================
// IntChain — Pre-allocated
// =============================================================================

type IntChain struct {
	v      *Validator
	field  string
	value  int
	exists bool
}

// Int initiates a validation chain for an integer value.
// Example:
//
//	v.Int("age", req.Age).Required().Min(18)
func (v *Validator) Int(field string, value int) *IntChain {
	v.i.field = field
	v.i.value = value
	v.i.exists = true
	if v.i.v == nil {
		v.i.v = v
	}
	return &v.i
}

// Required ensures the integer field was provided.
func (c *IntChain) Required() *IntChain {
	if c.v.err != nil {
		return c
	}
	if !c.exists {
		c.v.failRequired(c.field)
	}
	return c
}

// Min ensures the integer value is at least n.
func (c *IntChain) Min(n int) *IntChain {
	if c.v.err != nil {
		return c
	}
	if c.value < n {
		c.v.failMin(c.field, n)
	}
	return c
}

// Max ensures the integer value is at most n.
func (c *IntChain) Max(n int) *IntChain {
	if c.v.err != nil {
		return c
	}
	if c.value > n {
		c.v.failMax(c.field, n)
	}
	return c
}

// Equal ensures the integer value is exactly equal to the target.
func (c *IntChain) Equal(target int) *IntChain {
	if c.v.err != nil {
		return c
	}
	if c.value != target {
		c.v.failIntEqual(c.field, target)
	}
	return c
}

// NotEqual ensures the integer value is not equal to the target.
func (c *IntChain) NotEqual(target int) *IntChain {
	if c.v.err != nil {
		return c
	}
	if c.value == target {
		c.v.failIntNotEqual(c.field, target)
	}
	return c
}

// Positive ensures the integer value is greater than zero.
func (c *IntChain) Positive() *IntChain {
	if c.v.err != nil {
		return c
	}
	if c.value <= 0 {
		c.v.failPositive(c.field)
	}
	return c
}

// Custom allows for a custom validation function to be applied to the integer.
func (c *IntChain) Custom(fn func(int) bool, message string) *IntChain {
	if c.v.err != nil {
		return c
	}
	if !fn(c.value) {
		c.v.setError(constant.ERR_VALIDATION_CODE, c.field+" "+message, c.field)
	}
	return c
}

// GetErr returns the current validation error from the chain.
func (c *IntChain) GetErr() *model.BadRequestError { return c.v.err }

// =============================================================================
// Float64Chain — Pre-allocated
// =============================================================================

type Float64Chain struct {
	v      *Validator
	field  string
	value  float64
	exists bool
}

// Float64 initiates a validation chain for a float64 value.
// Example:
//
//	v.Float64("price", req.Price).Required().Positive()
func (v *Validator) Float64(field string, value float64) *Float64Chain {
	v.f.field = field
	v.f.value = value
	v.f.exists = true
	if v.f.v == nil {
		v.f.v = v
	}
	return &v.f
}

// Required ensures the float field was provided.
func (c *Float64Chain) Required() *Float64Chain {
	if c.v.err != nil {
		return c
	}
	if !c.exists {
		c.v.failRequired(c.field)
	}
	return c
}

// Min ensures the float value is at least n.
func (c *Float64Chain) Min(n float64) *Float64Chain {
	if c.v.err != nil {
		return c
	}
	if c.value < n {
		c.v.failFloatMin(c.field, n)
	}
	return c
}

// Max ensures the float value is at most n.
func (c *Float64Chain) Max(n float64) *Float64Chain {
	if c.v.err != nil {
		return c
	}
	if c.value > n {
		c.v.failFloatMax(c.field, n)
	}
	return c
}

// Positive ensures the float value is greater than zero.
func (c *Float64Chain) Positive() *Float64Chain {
	if c.v.err != nil {
		return c
	}
	if c.value <= 0 {
		c.v.failPositive(c.field)
	}
	return c
}

// GetErr returns the current validation error from the chain.
func (c *Float64Chain) GetErr() *model.BadRequestError { return c.v.err }

// =============================================================================
// Standalone Functions (For simpler use, slightly higher cost)
// =============================================================================

type Chain interface {
	GetErr() *model.BadRequestError
}

// String is a standalone helper that creates a new validator and starts a string chain.
func String(field, value string) *StringChain { return NewValidator().String(field, value) }

// Int is a standalone helper that creates a new validator and starts an int chain.
func Int(field string, value int) *IntChain { return NewValidator().Int(field, value) }

// Float64 is a standalone helper that creates a new validator and starts a float64 chain.
func Float64(field string, value float64) *Float64Chain {
	return NewValidator().Float64(field, value)
}

// Validate executes multiple validation chains and returns the first error encountered.
// Recommended for grouping independent validations.
func Validate(chains ...Chain) *model.BadRequestError {
	for _, c := range chains {
		if err := c.GetErr(); err != nil {
			return err
		}
	}
	return nil
}

// =============================================================================
// MultipartForm — Pre-allocated Validator
// =============================================================================

type MultipartForm struct {
	form      *multipart.Form
	validator Validator
}

// ParseMultipart parses a multipart/form-data request and returns a specialized validator.
// It handles internal validator initialization for zero-allocation parsing.
// Example:
//
//	form, err := util.ParseMultipart(r, constant.DEFAULT_MAX_MEMORY)
func ParseMultipart(r *http.Request, maxMemory int64) (*MultipartForm, error) {
	ct := r.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "multipart/form-data") {
		return nil, &model.BadRequestError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "request must be multipart/form-data",
			Param:   "Content-Type",
		}
	}

	if err := r.ParseMultipartForm(maxMemory); err != nil {
		return nil, err
	}
	m := &MultipartForm{
		form: r.MultipartForm,
	}
	// Initialise internal validator pointers
	m.validator.s.v = &m.validator
	m.validator.i.v = &m.validator
	m.validator.f.v = &m.validator
	return m, nil
}

// Close removes all temporary files created during multipart parsing.
func (m *MultipartForm) Close() {
	if m.form != nil {
		m.form.RemoveAll()
	}
}

// Err returns the first validation error encountered during form processing.
func (m *MultipartForm) Err() *model.BadRequestError {
	return m.validator.Err()
}

// String initiates a validation chain for a string form field.
func (m *MultipartForm) String(field string) *StringChain {
	values := m.form.Value[field]
	if len(values) == 0 {
		return m.validator.String(field, "")
	}
	return m.validator.String(field, values[0])
}

// Int initiates a validation chain for an integer form field.
func (m *MultipartForm) Int(field string) *IntChain {
	values := m.form.Value[field]
	if len(values) == 0 {
		i := &m.validator.i
		i.field, i.value, i.exists = field, 0, false
		return i
	}
	val := strings.TrimSpace(values[0])
	v, err := strconv.Atoi(val)
	if err != nil {
		m.validator.setError(constant.ERR_VALIDATION_CODE, field+" must be a valid integer", field)
		res := &m.validator.i
		res.field, res.exists = field, true
		return res
	}
	return m.validator.Int(field, v)
}

// Float64 initiates a validation chain for a float form field.
func (m *MultipartForm) Float64(field string) *Float64Chain {
	values := m.form.Value[field]
	if len(values) == 0 {
		f := &m.validator.f
		f.field, f.value, f.exists = field, 0, false
		return f
	}
	val := strings.TrimSpace(values[0])
	v, err := strconv.ParseFloat(val, 64)
	if err != nil {
		m.validator.setError(constant.ERR_VALIDATION_CODE, field+" must be a valid number", field)
		res := &m.validator.f
		res.field, res.exists = field, true
		return res
	}
	return m.validator.Float64(field, v)
}

// File returns a validation chain for a single file upload field.
func (m *MultipartForm) File(field string) *FileChain {
	headers := m.form.File[field]
	var header *multipart.FileHeader
	if len(headers) > 0 {
		header = headers[0]
	}
	return &FileChain{v: &m.validator, field: field, header: header}
}

// Files returns validation chains for a multiple file upload field.
func (m *MultipartForm) Files(field string) []*FileChain {
	headers := m.form.File[field]
	if len(headers) == 0 {
		return nil
	}
	chains := make([]*FileChain, len(headers))
	for i, h := range headers {
		chains[i] = &FileChain{v: &m.validator, field: field, header: h}
	}
	return chains
}

// =============================================================================
// FileChain — Keep as value for now (rarely used in chains)
// =============================================================================

type FileChain struct {
	v          *Validator
	field      string
	header     *multipart.FileHeader
	mimeCached bool
	mimeCache  string
}

// File initiates a standalone validation chain for a multipart.FileHeader pointer.
// Example: v.File("avatar", payload.Avatar).Required().MaxSize(2 * 1024 * 1024)
func (v *Validator) File(field string, file *multipart.FileHeader) *FileChain {
	return &FileChain{v: v, field: field, header: file}
}

// Required ensures that at least one file was uploaded for this field.
func (c *FileChain) Required() *FileChain {
	if c.v.err != nil {
		return c
	}
	if c.header == nil {
		c.v.failRequired(c.field)
	}
	return c
}

// MaxSize ensures the uploaded file size is at most n bytes.
func (c *FileChain) MaxSize(n int64) *FileChain {
	if c.v.err != nil || c.header == nil {
		return c
	}
	if c.header.Size > n {
		c.v.setError(constant.ERR_VALIDATION_CODE, c.field+" must be at most "+formatBytes(n), c.field)
	}
	return c
}

// MinSize ensures the uploaded file size is at least n bytes.
func (c *FileChain) MinSize(n int64) *FileChain {
	if c.v.err != nil || c.header == nil {
		return c
	}
	if c.header.Size < n {
		c.v.setError(constant.ERR_VALIDATION_CODE, c.field+" must be at least "+formatBytes(n), c.field)
	}
	return c
}

// AllowTypes ensures the uploaded file matches one of the provided MIME types.
func (c *FileChain) AllowTypes(mimeTypes ...string) *FileChain {
	if c.v.err != nil || c.header == nil {
		return c
	}
	detected, err := c.detectMIME()
	if err != nil {
		c.v.setError(constant.ERR_VALIDATION_CODE, c.field+" could not be read", c.field)
		return c
	}
	normalised := fastNormaliseMIME(detected)
	for _, mime := range mimeTypes {
		if normalised == fastNormaliseMIME(mime) {
			return c
		}
	}
	c.v.setError(constant.ERR_VALIDATION_CODE, c.field+" type not allowed", c.field)
	return c
}

func (c *FileChain) AllowExts(exts ...string) *FileChain {
	if c.v.err != nil || c.header == nil {
		return c
	}
	ext := strings.ToLower(filepath.Ext(c.header.Filename))
	for _, e := range exts {
		if ext == strings.ToLower(e) {
			return c
		}
	}
	c.v.setError(constant.ERR_VALIDATION_CODE, c.field+" extension not allowed", c.field)
	return c
}

// Custom allows for a custom validation function to be applied to the file header.
func (c *FileChain) Custom(fn func(*multipart.FileHeader) bool, message string) *FileChain {
	if c.v.err != nil || c.header == nil {
		return c
	}
	if !fn(c.header) {
		c.v.setError(constant.ERR_VALIDATION_CODE, c.field+" "+message, c.field)
	}
	return c
}

// Open opens the uploaded file for reading.
func (c *FileChain) Open() (multipart.File, *multipart.FileHeader, error) {
	if c.header == nil {
		return nil, nil, &model.BadRequestError{Code: constant.ERR_VALIDATION_CODE, Message: c.field + " is required", Param: c.field}
	}
	f, err := c.header.Open()
	if err != nil {
		return nil, nil, err
	}
	return f, c.header, nil
}

// Header returns the internal multipart.FileHeader.
func (c *FileChain) Header() *multipart.FileHeader { return c.header }

// GetErr returns the current validation error from the chain.
func (c *FileChain) GetErr() *model.BadRequestError { return c.v.err }

func (c *FileChain) detectMIME() (string, error) {
	if c.mimeCached {
		return c.mimeCache, nil
	}
	f, err := c.header.Open()
	if err != nil {
		return "", err
	}
	defer f.Close()

	var buf [512]byte
	n, _ := f.Read(buf[:])
	c.mimeCached = true
	c.mimeCache = http.DetectContentType(buf[:n])
	return c.mimeCache, nil
}

// =============================================================================
// Helper Functions (Optimized)
// =============================================================================

func isWhitespace(s string) bool {
	if len(s) == 0 {
		return true
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c > unicode.MaxASCII {
			goto nonASCII
		}
		if c != ' ' && c != '\t' && c != '\n' && c != '\r' && c != '\v' && c != '\f' {
			return false
		}
	}
	return true
nonASCII:
	for _, r := range s {
		if !unicode.IsSpace(r) {
			return false
		}
	}
	return true
}

func fastNormaliseMIME(mime string) string {
	idx := strings.IndexByte(mime, ';')
	if idx >= 0 {
		mime = mime[:idx]
	}
	return strings.ToLower(strings.TrimSpace(mime))
}

func formatBytes(n int64) string {
	const (
		kb = 1024
		mb = 1024 * kb
		gb = 1024 * mb
	)
	switch {
	case n >= gb:
		return strconv.FormatFloat(float64(n)/float64(gb), 'f', 1, 64) + "GB"
	case n >= mb:
		return strconv.FormatFloat(float64(n)/float64(mb), 'f', 1, 64) + "MB"
	case n >= kb:
		return strconv.FormatFloat(float64(n)/float64(kb), 'f', 1, 64) + "KB"
	default:
		return strconv.FormatInt(n, 10) + " bytes"
	}
}

// =============================================================================
// Image validation & conversion
// =============================================================================

var validImageExts = map[string]bool{
	".jpg": true, ".jpeg": true, ".png": true, ".gif": true, ".webp": true,
}

var allowedImageMIME = map[string]bool{
	"image/jpeg": true, "image/png": true, "image/gif": true, "image/webp": true,
}

// ValidateImage performs comprehensive validation on an uploaded image,
// checking size, extension, and actual MIME type, then converts it to WebP format.
// Returns a Reader for the optimized WebP data.
func ValidateImage(ctx context.Context, fileHeader *multipart.FileHeader, fieldName string) (*bytes.Reader, int64, error) {
	if err := ctx.Err(); err != nil {
		return nil, 0, err
	}

	if fileHeader.Size > constant.MAX_FILE_SIZE {
		return nil, 0, &model.BadRequestError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "image size exceeded " + strconv.FormatInt(constant.MAX_FILE_SIZE/(1024*1024), 10) + "MB limit",
			Param:   fieldName,
		}
	}

	ext := strings.ToLower(filepath.Ext(fileHeader.Filename))
	if !validImageExts[ext] {
		return nil, 0, &model.BadRequestError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "invalid file extension: " + ext,
			Param:   fieldName,
		}
	}

	f, err := fileHeader.Open()
	if err != nil {
		return nil, 0, &model.BadRequestError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: fieldName + " could not be opened",
			Param:   fieldName,
		}
	}
	defer func() { _ = f.Close() }()

	var sniff [512]byte
	n, _ := f.Read(sniff[:])
	detected := fastNormaliseMIME(http.DetectContentType(sniff[:n]))
	if !allowedImageMIME[detected] {
		return nil, 0, &model.BadRequestError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "invalid image type: " + detected,
			Param:   fieldName,
		}
	}

	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return nil, 0, err
	}

	buf := getBuffer()
	defer putBuffer(buf)

	if _, err := io.Copy(buf, io.LimitReader(f, constant.MAX_FILE_SIZE+1)); err != nil {
		return nil, 0, err
	}

	if int64(buf.Len()) > constant.MAX_FILE_SIZE {
		return nil, 0, errors.New("file too large")
	}

	webpBytes, err := ConvertToWebP(buf.Bytes(), 75, 512, 512)
	if err != nil {
		return nil, 0, err
	}

	return bytes.NewReader(webpBytes), int64(len(webpBytes)), nil
}

// ExtractAndValidateImage extracts multipart file header from fiber.Ctx,
// validates size/extension/MIME, converts to WebP. Returns reader + final size.
func ExtractAndValidateImage(ctx fiber.Ctx, ctxContext context.Context, fieldName string) (*bytes.Reader, int64, error) {
	fileHeader, err := ctx.FormFile(fieldName)
	if err != nil {
		return nil, 0, &model.BadRequestError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: fieldName + " is required",
			Param:   fieldName,
		}
	}
	return ValidateImage(ctxContext, fileHeader, fieldName)
}

// ConvertToWebP converts image data to WebP format using bimg.
// Supports quality adjustment and optional resizing with cropping.
func ConvertToWebP(data []byte, quality, maxW, maxH int) ([]byte, error) {
	if quality < 1 || quality > 100 || maxW < 0 || maxH < 0 {
		return nil, ErrInvalidImage
	}

	img := bimg.NewImage(data)
	size, err := img.Size()
	if err != nil {
		return nil, err
	}

	opts := bimg.Options{
		Quality: quality,
		Type:    bimg.WEBP,
	}

	if maxW > 0 && maxH > 0 && (size.Width > maxW || size.Height > maxH) {
		opts.Width = maxW
		opts.Height = maxH
		opts.Crop = true
	}

	res, err := img.Process(opts)
	if err != nil {
		return nil, ErrImageProcessing
	}
	return res, nil
}
