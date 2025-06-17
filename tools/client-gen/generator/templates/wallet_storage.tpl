{{- define "printArgs" }}
    {{- range (.Arguments.SkipTypes "AuthID") }}
        {{- .Name }} {{ .Type | printType }},
    {{- end }}
{{- end }}

package {{ .Package.Name }}

{{- if .Imports }}
    import (
    {{- range .Imports }}
        {{ . }}
    {{- end }}
    )
{{- end }}

{{ .Package.OriginalPkgImportStatement }}

{{- with index .Interfaces 0 }}
    type WalletStorageBasic interface {
    {{- range .Methods }}
        {{- range .Comments }}
            // {{ . }}
        {{- end }}
        {{- range .Annotations }}
            // {{ . }}
        {{- end }}
        {{ .Name }}( {{- template "printArgs" . }} )  ({{ range .Results }} {{ .Type | printType }}, {{- end }})

    {{- end }}
    }
{{- end}}
