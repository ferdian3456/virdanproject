package shared

import "embed"

//go:embed template/*.html
var TemplateFS embed.FS
