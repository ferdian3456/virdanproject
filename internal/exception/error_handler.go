package exception

import (
	"fmt"
	"runtime/debug"
	"strings"

	"github.com/ferdian3456/virdanproject/internal/constant"
	"github.com/gofiber/fiber/v3"
	"go.uber.org/zap"
)

func Recovery(log *zap.Logger) fiber.Handler {
	return func(ctx fiber.Ctx) error {
		defer func() {
			if r := recover(); r != nil {
				var errMsg string
				switch v := r.(type) {
				case error:
					errMsg = v.Error()
				case string:
					errMsg = v
				default:
					errMsg = fmt.Sprintf("%v", v)
				}

				stack := debug.Stack()
				panicSource := parsePanicSource(stack)

				log.WithOptions(zap.WithCaller(false)).Error("panic occurred and recovered",
					zap.String("caller", panicSource),
					zap.String("error", errMsg),
				)

				_ = ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
					"error": fiber.Map{
						"code":    constant.ERR_INTERNAL_SERVER_ERROR_CODE,
						"message": constant.ERR_INTENRAL_SERVER_ERROR_MESSAGE,
						"param":   "",
					},
				})
			}
		}()

		return ctx.Next()
	}
}

func parsePanicSource(stack []byte) string {
	lines := strings.Split(string(stack), "\n")

	panicIdx := -1
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "panic(") {
			panicIdx = i
			break
		}
	}

	if panicIdx == -1 {
		return "unknown"
	}

	for i := panicIdx + 1; i < len(lines); i++ {
		if !strings.HasPrefix(lines[i], "\t") {
			continue
		}
		fileLine := strings.TrimSpace(lines[i])
		if idx := strings.LastIndex(fileLine, " +0x"); idx != -1 {
			fileLine = fileLine[:idx]
		}
		short := shortPath(fileLine)
		if strings.HasPrefix(short, "runtime/") {
			continue
		}
		return short
	}

	return "unknown"
}

func shortPath(path string) string {
	parts := strings.Split(path, "/")
	if len(parts) >= 2 {
		return strings.Join(parts[len(parts)-2:], "/")
	}
	return path
}
