package context

import "embed"

//go:embed templates/free5gc
var free5gcTemplateFS embed.FS

//go:embed templates/free5gc-ulcl
var free5gcUlclTemplateFS embed.FS

//go:embed templates/free5gc-ulcl-2slice
var free5gcUlcl2SliceTemplateFS embed.FS

//go:embed templates/gnb
var gnbTemplateFS embed.FS

//go:embed templates/gnb-slice1
var gnbSlice1TemplateFS embed.FS

//go:embed templates/gnb-slice2
var gnbSlice2TemplateFS embed.FS

//go:embed templates/ue
var ueTemplateFS embed.FS
