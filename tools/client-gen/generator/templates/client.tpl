package {{ .Package.Name }}

{{- if .Imports }}
    import (
    {{- range .Imports }}
        {{ . }}
    {{- end }}
    )
{{- end }}

{{ .Package.OriginalPkgImportStatement }}

{{- range .Interfaces }}
    {{- $clientName := printf "%sClient" .Name }}

    type {{ $clientName }} struct {
    client *rpc{{ .Name }}
    }

    {{- range .Methods }}
        {{- range .Comments }}
            // {{ . }}
        {{- end}}
        func (c *{{ $clientName }}) {{ .Name }}({{ range .Arguments }}{{ .Name }} {{ .Type | printType }}, {{ end }}) ({{ range .Results }}{{ .Type | printType }},{{ end }}) {
        {{ if gt (len .Results) 0 }} return {{ end }} c.client.{{ .Name }}({{ range .Arguments }}{{ .Name }}, {{ end }})
        }
    {{ end }}

    type rpc{{ .Name }} struct {
    {{- range .Methods }}
        {{ .Name }} func({{ range .Arguments }}{{ .Type | printType }}, {{ end }}) ({{ range .Results }}{{ .Type | printType }},{{ end }})
    {{- end }}
    }

{{- end }}
