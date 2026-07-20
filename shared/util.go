package shared

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math/big"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/bytedance/sonic"
	"github.com/gofiber/fiber/v3"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/h2non/bimg"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"gopkg.in/gomail.v2"
)

func EncodeCursor[T any](cursor T) string {
	b, err := sonic.Marshal(cursor)
	if err != nil {
		return ""
	}
	return base64.StdEncoding.EncodeToString(b)
}

func DecodeCursor[T any](encoded string) (*T, error) {
	b, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, err
	}
	var cursor T
	if err = sonic.Unmarshal(b, &cursor); err != nil {
		return nil, err
	}
	return &cursor, nil
}

func GenerateOTP() (string, error) {
	const digits = "0123456789"
	const length = 6

	b := make([]byte, length)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}

	for i := range b {
		b[i] = digits[b[i]%10]
	}

	return string(b), nil
}

func GenerateShortName(name string) string {
	name = strings.TrimSpace(name)
	if len(name) == 0 {
		return ""
	}

	runes := []rune(name)
	if len(runes) > 12 {
		return string(runes[:12])
	}

	return name
}

func FormatRemainingTime(seconds int64) string {
	if seconds < 60 {
		return fmt.Sprintf("%d seconds", seconds)
	}
	minutes := seconds / 60
	remainingSecs := seconds % 60
	if remainingSecs == 0 {
		return fmt.Sprintf("%d minutes", minutes)
	}
	return fmt.Sprintf("%d minutes and %d seconds", minutes, remainingSecs)
}

func GenerateInviteCode() (string, error) {
	const digits = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	const length = 8
	b := make([]byte, length)

	for i := range b {
		num, err := rand.Int(rand.Reader, big.NewInt(int64(len(digits))))
		if err != nil {
			return "", err
		}
		b[i] = digits[num.Int64()]
	}
	return string(b), nil
}

func HashSHA256(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func SortUUIDPair(a, b string) (low, high string) {
	if a < b {
		return a, b
	}
	return b, a
}

func ReadRequestBody(ctx fiber.Ctx, result interface{}) error {
	err := ctx.Bind().Body(result)
	if err != nil {
		return &BadRequestError{
			Code:    ERR_BAD_REQUEST_CODE,
			Message: ERR_INVALID_REQUEST_BODY_MESSAGE,
		}
	}
	return nil
}

func SendSuccessResponseNoData(ctx fiber.Ctx) error {
	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
		"status": "OK",
	})
}

func SendSuccessResponseWithData(ctx fiber.Ctx, data interface{}) error {
	return ctx.Status(fiber.StatusOK).JSON(data)
}

func SendError(ctx fiber.Ctx, err error) error {
	statusCode := fiber.StatusInternalServerError
	errCode := ERR_INTERNAL_SERVER_ERROR_CODE
	message := err.Error()
	param := ""

	if apiErr, ok := err.(ApiError); ok {
		statusCode = apiErr.StatusCode()
		errCode = apiErr.GetCode()
		param = apiErr.GetParam()
	} else {
		statusCode = fiber.StatusInternalServerError
		errCode = ERR_INTERNAL_SERVER_ERROR_CODE
		message = ERR_INTENRAL_SERVER_ERROR_MESSAGE
		param = ""
	}

	ctx.Locals("handler_error", err)

	return ctx.Status(statusCode).JSON(fiber.Map{
		"error": fiber.Map{
			"code":    errCode,
			"message": message,
			"param":   param,
		},
	})
}

func ReadMultipartBody(ctx fiber.Ctx) error {
	ct := ctx.Get("Content-Type")
	if len(ct) < 19 || ct[:19] != "multipart/form-data" {
		return &BadRequestError{
			Code:    ERR_BAD_REQUEST_CODE,
			Message: ERR_INVALID_CONTENT_TYPE_MESSAGE,
			Param:   "Content-Type",
		}
	}
	return nil
}

var (
	BearerPrefix            = "Bearer "
	TokenIssuer             = "github.com/ferdian3456/virdanproject" // #nosec G101 -- JWT issuer string, not a credential
	AccessTokenDuration     = 15 * time.Minute
	RefreshTokenDuration    = 7 * 24 * time.Hour
	ErrInvalidSigningMethod = errors.New("invalid token signing method")
)

func HashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}

func GenerateAccessToken(userId string, jwtSecretKey string) (string, error) {
	if jwtSecretKey == "" {
		return "", errors.New("jwt secret key is not configured")
	}

	now := time.Now().UTC()
	claims := &Claims{
		UserId: userId,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(AccessTokenDuration)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    TokenIssuer,
			Subject:   fmt.Sprintf("user:%s", userId),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, err := token.SignedString([]byte(jwtSecretKey))
	if err != nil {
		return "", err
	}

	return signedToken, nil
}

func GenerateTokenPair(userId string, jwtSecretKey string) (TokenResponse, error) {
	accessToken, err := GenerateAccessToken(userId, jwtSecretKey)
	if err != nil {
		return TokenResponse{}, err
	}

	refreshToken := uuid.New().String()

	return TokenResponse{
		AccessToken:           accessToken,
		AccessTokenExpiresIn:  int(AccessTokenDuration.Seconds()),
		RefreshToken:          refreshToken,
		RefreshTokenExpiresIn: int(RefreshTokenDuration.Seconds()),
		TokenType:             "Bearer",
	}, nil
}

func ValidateAccessToken(tokenString string, jwtSecretKey string) (string, string, error) {
	if jwtSecretKey == "" {
		return "", "", errors.New("jwt secret key is not configured")
	}

	if tokenString == "" {
		return "", "", &UnauthorizedError{
			Code:    ERR_UNAUTHORIZED_ERROR,
			Message: "Authentication token is empty",
			Param:   "accessToken",
		}
	}

	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidSigningMethod
		}
		return []byte(jwtSecretKey), nil
	})

	if err != nil {
		return "", "", handleAuthParseError(err)
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return "", "", &UnauthorizedError{
			Code:    ERR_UNAUTHORIZED_ERROR,
			Message: "Authentication token is invalid",
			Param:   "accessToken",
		}
	}

	return tokenString, claims.UserId, nil
}

func handleAuthParseError(err error) error {
	switch {
	case errors.Is(err, jwt.ErrTokenMalformed):
		return &UnauthorizedError{
			Code:    ERR_UNAUTHORIZED_ERROR,
			Message: "Authentication token is malformed",
			Param:   "accessToken",
		}
	case errors.Is(err, jwt.ErrTokenExpired):
		return &UnauthorizedError{
			Code:    ERR_UNAUTHORIZED_ERROR,
			Message: "Authentication token is expired",
			Param:   "accessToken",
		}
	case errors.Is(err, jwt.ErrTokenNotValidYet):
		return &UnauthorizedError{
			Code:    ERR_UNAUTHORIZED_ERROR,
			Message: "Authentication token is not valid yet",
			Param:   "accessToken",
		}
	case errors.Is(err, ErrInvalidSigningMethod):
		return &UnauthorizedError{
			Code:    ERR_UNAUTHORIZED_ERROR,
			Message: "Authentication token has invalid signing method",
			Param:   "accessToken",
		}
	default:
		return &UnauthorizedError{
			Code:    ERR_UNAUTHORIZED_ERROR,
			Message: "Authentication token is invalid",
			Param:   "accessToken",
		}
	}
}

func GetLoggerWithTraceContext(ctx context.Context, logger *zap.Logger) *zap.Logger {
	span := trace.SpanFromContext(ctx)
	if span.SpanContext().IsValid() {
		return logger.With(
			zap.String("trace_id", span.SpanContext().TraceID().String()),
			zap.String("span_id", span.SpanContext().SpanID().String()),
		)
	}
	return logger
}

func RecordErrorTelemetry(ctx context.Context, span trace.Span, err error) {
	if span == nil {
		span = trace.SpanFromContext(ctx)
	}
	if span == nil || !span.SpanContext().IsValid() {
		return
	}

	opts := []trace.EventOption{}
	if apiErr, ok := err.(ApiError); ok {
		opts = append(opts, trace.WithAttributes(
			attribute.String("error.code", apiErr.GetCode()),
			attribute.String("error.param", apiErr.GetParam()),
		))
	}

	span.RecordError(err, opts...)
	span.SetStatus(codes.Error, err.Error())
}

func ToPtr[T any](v T) *T {
	return &v
}

func Deref[T any](ptr *T, def T) T {
	if ptr != nil {
		return *ptr
	}
	return def
}

func Equal[T comparable](a, b *T) bool {
	if (a == nil) != (b == nil) {
		return false
	}
	if a == nil {
		return true
	}
	return *a == *b
}

func SendEmail(smtpHost string, smtpPort int, senderName string, senderEmail string, senderPassowrd string, receiverEmail string, subject string, body string) error {
	mailer := gomail.NewMessage()
	mailer.SetHeader("From", senderName)
	mailer.SetHeader("To", receiverEmail)
	mailer.SetHeader("Subject", subject)
	mailer.SetBody("text/html", body)

	dialer := gomail.NewDialer(
		smtpHost,
		smtpPort,
		senderEmail,
		senderPassowrd,
	)

	err := dialer.DialAndSend(mailer)
	if err != nil {
		return err
	}

	return nil
}

func TruncateRunes(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max])
}

var (
	ErrInvalidImage    = errors.New("invalid image parameters")
	ErrImageProcessing = errors.New("failed to process image")
)

var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)

var (
	nicknameRegex     = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
	usernameRegex     = regexp.MustCompile(`^[a-zA-Z0-9_.]+$`)
	UsernameRegex     = usernameRegex
	UsernameErrorText = "Username may only contain letters, digits, underscores and dots"
)

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

type Validator struct {
	err *BadRequestError
	s   StringChain
	i   IntChain
	f   Float64Chain
}

func NewValidator() *Validator {
	v := &Validator{}
	v.s.v = v
	v.i.v = v
	v.f.v = v
	return v
}

func (v *Validator) Reset() {
	v.err = nil
}

func (v *Validator) Err() *BadRequestError {
	return v.err
}

func (v *Validator) Validate() error {
	if v.err == nil {
		return nil
	}
	return v.err
}

func (v *Validator) setError(code, message, param string) {
	if v.err == nil {
		v.err = &BadRequestError{
			Code:    code,
			Message: message,
			Param:   param,
		}
	}
}

func (v *Validator) failRequired(field string) {
	v.setError(ERR_VALIDATION_CODE, field+" is required", field)
}

func (v *Validator) failMinLen(field string, n int) {
	v.setError(ERR_VALIDATION_CODE, field+" must be at least "+strconv.Itoa(n)+" characters", field)
}

func (v *Validator) failMaxLen(field string, n int) {
	v.setError(ERR_VALIDATION_CODE, field+" must be at most "+strconv.Itoa(n)+" characters", field)
}

func (v *Validator) failExactLen(field string, n int) {
	v.setError(ERR_VALIDATION_CODE, field+" must be exactly "+strconv.Itoa(n)+" characters", field)
}

func (v *Validator) failEqual(field, targetName string) {
	v.setError(ERR_VALIDATION_CODE, field+" must be equal to "+targetName, field)
}

func (v *Validator) failNotEqual(field, targetName string) {
	v.setError(ERR_VALIDATION_CODE, field+" must not be equal to "+targetName, field)
}

func (v *Validator) failMin(field string, n int) {
	v.setError(ERR_VALIDATION_CODE, field+" must be at least "+strconv.Itoa(n), field)
}

func (v *Validator) failMax(field string, n int) {
	v.setError(ERR_VALIDATION_CODE, field+" must be at most "+strconv.Itoa(n), field)
}

func (v *Validator) failPositive(field string) {
	v.setError(ERR_VALIDATION_CODE, field+" must be positive", field)
}

func (v *Validator) failIntEqual(field string, target int) {
	v.setError(ERR_VALIDATION_CODE, field+" must be exactly "+strconv.Itoa(target), field)
}

func (v *Validator) failIntNotEqual(field string, target int) {
	v.setError(ERR_VALIDATION_CODE, field+" must not be "+strconv.Itoa(target), field)
}

func (v *Validator) failFloatMin(field string, n float64) {
	v.setError(ERR_VALIDATION_CODE, field+" must be at least "+strconv.FormatFloat(n, 'f', -1, 64), field)
}

func (v *Validator) failFloatMax(field string, n float64) {
	v.setError(ERR_VALIDATION_CODE, field+" must be at most "+strconv.FormatFloat(n, 'f', -1, 64), field)
}

type StringChain struct {
	v     *Validator
	field string
	value string
}

func (v *Validator) UUID(field, value string) *StringChain {
	return v.String(field, value).UUID()
}

func (v *Validator) String(field, value string) *StringChain {
	v.s.field = field
	v.s.value = value
	if v.s.v == nil {
		v.s.v = v
	}
	return &v.s
}

func (c *StringChain) Required() *StringChain {
	if c.v.err != nil {
		return c
	}
	if isWhitespace(c.value) {
		c.v.failRequired(c.field)
	}
	return c
}

func (c *StringChain) MinLen(n int) *StringChain {
	if c.v.err != nil {
		return c
	}
	if utf8.RuneCountInString(c.value) < n {
		c.v.failMinLen(c.field, n)
	}
	return c
}

func (c *StringChain) MaxLen(n int) *StringChain {
	if c.v.err != nil {
		return c
	}
	if utf8.RuneCountInString(c.value) > n {
		c.v.failMaxLen(c.field, n)
	}
	return c
}

func (c *StringChain) Len(n int) *StringChain {
	if c.v.err != nil {
		return c
	}
	if utf8.RuneCountInString(c.value) != n {
		c.v.failExactLen(c.field, n)
	}
	return c
}

func (c *StringChain) Email() *StringChain {
	if c.v.err != nil {
		return c
	}
	if !emailRegex.MatchString(c.value) {
		c.v.setError(ERR_VALIDATION_CODE, c.field+" must be a valid email address", c.field)
	}
	return c
}

func (c *StringChain) Equal(target, targetName string) *StringChain {
	if c.v.err != nil {
		return c
	}
	if c.value != target {
		c.v.failEqual(c.field, targetName)
	}
	return c
}

func (c *StringChain) NotEqual(target, targetName string) *StringChain {
	if c.v.err != nil {
		return c
	}
	if c.value == target {
		c.v.failNotEqual(c.field, targetName)
	}
	return c
}

func (c *StringChain) OneOf(options ...string) *StringChain {
	if c.v.err != nil {
		return c
	}
	for _, opt := range options {
		if c.value == opt {
			return c
		}
	}
	c.v.setError(ERR_VALIDATION_CODE, c.field+" must be one of: "+strings.Join(options, ", "), c.field)
	return c
}

func (c *StringChain) Custom(fn func(string) bool, message string) *StringChain {
	if c.v.err != nil {
		return c
	}
	if !fn(c.value) {
		c.v.setError(ERR_VALIDATION_CODE, c.field+" "+message, c.field)
	}
	return c
}

func (c *StringChain) Nickname() *StringChain {
	if c.v.err != nil || c.value == "" {
		return c
	}
	if !nicknameRegex.MatchString(c.value) {
		c.v.setError(ERR_VALIDATION_CODE, c.field+" only allows letters, digits, underscore, dash", c.field)
	}
	return c
}

func (c *StringChain) Regex(re *regexp.Regexp, message string) *StringChain {
	if c.v.err != nil || c.value == "" {
		return c
	}
	if !re.MatchString(c.value) {
		c.v.setError(ERR_VALIDATION_CODE, message, c.field)
	}
	return c
}

func (c *StringChain) UUID() *StringChain {
	if c.v.err != nil || c.value == "" {
		return c
	}
	if _, err := uuid.Parse(c.value); err != nil {
		c.v.setError(ERR_VALIDATION_CODE, c.field+" is not a valid UUID", c.field)
	}
	return c
}

func (c *StringChain) GetErr() *BadRequestError { return c.v.err }

type IntChain struct {
	v      *Validator
	field  string
	value  int
	exists bool
}

func (v *Validator) Int(field string, value int) *IntChain {
	v.i.field = field
	v.i.value = value
	v.i.exists = true
	if v.i.v == nil {
		v.i.v = v
	}
	return &v.i
}

func (c *IntChain) Required() *IntChain {
	if c.v.err != nil {
		return c
	}
	if !c.exists {
		c.v.failRequired(c.field)
	}
	return c
}

func (c *IntChain) Min(n int) *IntChain {
	if c.v.err != nil {
		return c
	}
	if c.value < n {
		c.v.failMin(c.field, n)
	}
	return c
}

func (c *IntChain) Max(n int) *IntChain {
	if c.v.err != nil {
		return c
	}
	if c.value > n {
		c.v.failMax(c.field, n)
	}
	return c
}

func (c *IntChain) Equal(target int) *IntChain {
	if c.v.err != nil {
		return c
	}
	if c.value != target {
		c.v.failIntEqual(c.field, target)
	}
	return c
}

func (c *IntChain) NotEqual(target int) *IntChain {
	if c.v.err != nil {
		return c
	}
	if c.value == target {
		c.v.failIntNotEqual(c.field, target)
	}
	return c
}

func (c *IntChain) Positive() *IntChain {
	if c.v.err != nil {
		return c
	}
	if c.value <= 0 {
		c.v.failPositive(c.field)
	}
	return c
}

func (c *IntChain) Custom(fn func(int) bool, message string) *IntChain {
	if c.v.err != nil {
		return c
	}
	if !fn(c.value) {
		c.v.setError(ERR_VALIDATION_CODE, c.field+" "+message, c.field)
	}
	return c
}

func (c *IntChain) GetErr() *BadRequestError { return c.v.err }

type Float64Chain struct {
	v      *Validator
	field  string
	value  float64
	exists bool
}

func (v *Validator) Float64(field string, value float64) *Float64Chain {
	v.f.field = field
	v.f.value = value
	v.f.exists = true
	if v.f.v == nil {
		v.f.v = v
	}
	return &v.f
}

func (c *Float64Chain) Required() *Float64Chain {
	if c.v.err != nil {
		return c
	}
	if !c.exists {
		c.v.failRequired(c.field)
	}
	return c
}

func (c *Float64Chain) Min(n float64) *Float64Chain {
	if c.v.err != nil {
		return c
	}
	if c.value < n {
		c.v.failFloatMin(c.field, n)
	}
	return c
}

func (c *Float64Chain) Max(n float64) *Float64Chain {
	if c.v.err != nil {
		return c
	}
	if c.value > n {
		c.v.failFloatMax(c.field, n)
	}
	return c
}

func (c *Float64Chain) Positive() *Float64Chain {
	if c.v.err != nil {
		return c
	}
	if c.value <= 0 {
		c.v.failPositive(c.field)
	}
	return c
}

func (c *Float64Chain) GetErr() *BadRequestError { return c.v.err }

type Chain interface {
	GetErr() *BadRequestError
}

func String(field, value string) *StringChain { return NewValidator().String(field, value) }

func Int(field string, value int) *IntChain { return NewValidator().Int(field, value) }

func Float64(field string, value float64) *Float64Chain {
	return NewValidator().Float64(field, value)
}

func Validate(chains ...Chain) *BadRequestError {
	for _, c := range chains {
		if err := c.GetErr(); err != nil {
			return err
		}
	}
	return nil
}

type MultipartForm struct {
	form      *multipart.Form
	validator Validator
}

func ParseMultipart(r *http.Request, maxMemory int64) (*MultipartForm, error) {
	ct := r.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "multipart/form-data") {
		return nil, &BadRequestError{
			Code:    ERR_VALIDATION_CODE,
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
	m.validator.s.v = &m.validator
	m.validator.i.v = &m.validator
	m.validator.f.v = &m.validator
	return m, nil
}

func (m *MultipartForm) Close() {
	if m.form != nil {
		m.form.RemoveAll()
	}
}

func (m *MultipartForm) Err() *BadRequestError {
	return m.validator.Err()
}

func (m *MultipartForm) String(field string) *StringChain {
	values := m.form.Value[field]
	if len(values) == 0 {
		return m.validator.String(field, "")
	}
	return m.validator.String(field, values[0])
}

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
		m.validator.setError(ERR_VALIDATION_CODE, field+" must be a valid integer", field)
		res := &m.validator.i
		res.field, res.exists = field, true
		return res
	}
	return m.validator.Int(field, v)
}

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
		m.validator.setError(ERR_VALIDATION_CODE, field+" must be a valid number", field)
		res := &m.validator.f
		res.field, res.exists = field, true
		return res
	}
	return m.validator.Float64(field, v)
}

func (m *MultipartForm) File(field string) *FileChain {
	headers := m.form.File[field]
	var header *multipart.FileHeader
	if len(headers) > 0 {
		header = headers[0]
	}
	return &FileChain{v: &m.validator, field: field, header: header}
}

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

type FileChain struct {
	v          *Validator
	field      string
	header     *multipart.FileHeader
	mimeCached bool
	mimeCache  string
}

func (v *Validator) File(field string, file *multipart.FileHeader) *FileChain {
	return &FileChain{v: v, field: field, header: file}
}

func (c *FileChain) Required() *FileChain {
	if c.v.err != nil {
		return c
	}
	if c.header == nil {
		c.v.failRequired(c.field)
	}
	return c
}

func (c *FileChain) MaxSize(n int64) *FileChain {
	if c.v.err != nil || c.header == nil {
		return c
	}
	if c.header.Size > n {
		c.v.setError(ERR_VALIDATION_CODE, c.field+" must be at most "+formatBytes(n), c.field)
	}
	return c
}

func (c *FileChain) MinSize(n int64) *FileChain {
	if c.v.err != nil || c.header == nil {
		return c
	}
	if c.header.Size < n {
		c.v.setError(ERR_VALIDATION_CODE, c.field+" must be at least "+formatBytes(n), c.field)
	}
	return c
}

func (c *FileChain) AllowTypes(mimeTypes ...string) *FileChain {
	if c.v.err != nil || c.header == nil {
		return c
	}
	detected, err := c.detectMIME()
	if err != nil {
		c.v.setError(ERR_VALIDATION_CODE, c.field+" could not be read", c.field)
		return c
	}
	normalised := fastNormaliseMIME(detected)
	for _, mime := range mimeTypes {
		if normalised == fastNormaliseMIME(mime) {
			return c
		}
	}
	c.v.setError(ERR_VALIDATION_CODE, c.field+" type not allowed", c.field)
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
	c.v.setError(ERR_VALIDATION_CODE, c.field+" extension not allowed", c.field)
	return c
}

func (c *FileChain) Custom(fn func(*multipart.FileHeader) bool, message string) *FileChain {
	if c.v.err != nil || c.header == nil {
		return c
	}
	if !fn(c.header) {
		c.v.setError(ERR_VALIDATION_CODE, c.field+" "+message, c.field)
	}
	return c
}

func (c *FileChain) Open() (multipart.File, *multipart.FileHeader, error) {
	if c.header == nil {
		return nil, nil, &BadRequestError{Code: ERR_VALIDATION_CODE, Message: c.field + " is required", Param: c.field}
	}
	f, err := c.header.Open()
	if err != nil {
		return nil, nil, err
	}
	return f, c.header, nil
}

func (c *FileChain) Header() *multipart.FileHeader { return c.header }

func (c *FileChain) GetErr() *BadRequestError { return c.v.err }

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

var validImageExts = map[string]bool{
	".jpg": true, ".jpeg": true, ".png": true, ".gif": true, ".webp": true,
}

var allowedImageMIME = map[string]bool{
	"image/jpeg": true, "image/png": true, "image/gif": true, "image/webp": true,
}

func ValidateImage(ctx context.Context, fileHeader *multipart.FileHeader, fieldName string, maxSize int64, maxW, maxH int, crop bool) (*bytes.Reader, int64, int, int, error) {
	if err := ctx.Err(); err != nil {
		return nil, 0, 0, 0, err
	}

	if fileHeader.Size > maxSize {
		return nil, 0, 0, 0, &BadRequestError{
			Code:    ERR_UPLOAD_SIZE_EXCEEDED_CODE,
			Message: "image size exceeded " + strconv.FormatInt(maxSize/(1024*1024), 10) + "MB limit",
			Param:   fieldName,
		}
	}

	ext := strings.ToLower(filepath.Ext(fileHeader.Filename))
	if !validImageExts[ext] {
		return nil, 0, 0, 0, &BadRequestError{
			Code:    ERR_VALIDATION_CODE,
			Message: "invalid file extension: " + ext,
			Param:   fieldName,
		}
	}

	f, err := fileHeader.Open()
	if err != nil {
		return nil, 0, 0, 0, &BadRequestError{
			Code:    ERR_VALIDATION_CODE,
			Message: fieldName + " could not be opened",
			Param:   fieldName,
		}
	}
	defer func() { _ = f.Close() }()

	var sniff [512]byte
	n, _ := f.Read(sniff[:])
	detected := fastNormaliseMIME(http.DetectContentType(sniff[:n]))
	if !allowedImageMIME[detected] {
		return nil, 0, 0, 0, &BadRequestError{
			Code:    ERR_VALIDATION_CODE,
			Message: "invalid image type: " + detected,
			Param:   fieldName,
		}
	}

	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return nil, 0, 0, 0, err
	}

	buf := getBuffer()
	defer putBuffer(buf)

	if _, err := io.Copy(buf, io.LimitReader(f, maxSize+1)); err != nil {
		return nil, 0, 0, 0, err
	}

	if int64(buf.Len()) > maxSize {
		return nil, 0, 0, 0, errors.New("file too large")
	}

	webpBytes, w, h, err := ConvertToWebP(buf.Bytes(), 75, maxW, maxH, crop)
	if err != nil {
		return nil, 0, 0, 0, err
	}

	return bytes.NewReader(webpBytes), int64(len(webpBytes)), w, h, nil
}

func ExtractAndValidateImage(ctx fiber.Ctx, ctxContext context.Context, fieldName string, maxSize int64, maxW, maxH int, crop bool) (*bytes.Reader, int64, int, int, error) {
	fileHeader, err := ctx.FormFile(fieldName)
	if err != nil {
		return nil, 0, 0, 0, &BadRequestError{
			Code:    ERR_VALIDATION_CODE,
			Message: fieldName + " is required",
			Param:   fieldName,
		}
	}
	return ValidateImage(ctxContext, fileHeader, fieldName, maxSize, maxW, maxH, crop)
}

func ConvertToWebP(data []byte, quality, maxW, maxH int, crop bool) ([]byte, int, int, error) {
	if quality < 1 || quality > 100 || maxW < 0 || maxH < 0 {
		return nil, 0, 0, ErrInvalidImage
	}

	img := bimg.NewImage(data)

	if rotated, err := img.AutoRotate(); err == nil {
		img = bimg.NewImage(rotated)
	}

	size, err := img.Size()
	if err != nil {
		return nil, 0, 0, err
	}

	opts := bimg.Options{
		Quality:      quality,
		Type:         bimg.WEBP,
		NoAutoRotate: true,
	}

	if maxW > 0 && maxH > 0 && (size.Width > maxW || size.Height > maxH) {
		if crop {
			opts.Width = maxW
			opts.Height = maxH
			opts.Crop = true
		} else {
			scaleW := float64(maxW) / float64(size.Width)
			scaleH := float64(maxH) / float64(size.Height)
			scale := scaleW
			if scaleH < scaleW {
				scale = scaleH
			}
			opts.Width = int(float64(size.Width) * scale)
			opts.Height = int(float64(size.Height) * scale)
			opts.Crop = false
		}
	}

	res, err := img.Process(opts)
	if err != nil {
		return nil, 0, 0, ErrImageProcessing
	}

	outputSize, err := bimg.NewImage(res).Size()
	if err != nil {
		return nil, 0, 0, ErrImageProcessing
	}
	return res, outputSize.Width, outputSize.Height, nil
}
