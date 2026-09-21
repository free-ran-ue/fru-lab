package context

import "embed"

//go:embed templates/free5gc
var free5gcTemplateFS embed.FS

//go:embed templates/free5gc-ulcl
var free5gcUlclTemplateFS embed.FS

//go:embed templates/gnb
var gnbTemplateFS embed.FS

//go:embed templates/ue
var ueTemplateFS embed.FS
