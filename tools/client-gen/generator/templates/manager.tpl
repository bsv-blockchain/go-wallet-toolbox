{{- define "printFunctionArgs" }}
    {{- range (.Arguments.SkipTypes "AuthID") }}
        {{- .Name }} {{ .Type | printType }},
    {{- end }}
{{- end }}

{{- define "ReaderWriter" }}
    {{- if . }}
        {{- "Writer" -}}
    {{- else}}
        {{- "Reader" -}}
    {{end}}
{{- end }}
{{- $pkg := .Package }}
package {{ $pkg.Name }}

{{- if .Imports }}
    import (
    {{- range .Imports }}
        {{ . }}
    {{- end }}
    "fmt"
    )
{{- end }}

{{ $pkg.OriginalPkgImportStatement }}

{{- range .Interfaces }}
    {{- $managerName := "WalletStorageManager" }}

    {{- range .Methods }}
        {{- range .Comments }}
            // {{ . }}
        {{- end}}
        func (m *{{ $managerName }}) {{ .Name }}({{- template "printFunctionArgs" . }}) ({{ range .Results }}{{ .Type | printType }},{{ end }}) {
        {{- $authIDVarName := .Arguments.ArgumentOfType "AuthID" }}
        {{- $contextVarName := .Arguments.ArgumentOfType "context.Context" }}
        {{- if $authIDVarName }}
            {{ $authIDVarName }}, err := m.GetAuth({{ coalesce $contextVarName "context.Background()" }})
            {{- if .Results.HasError }}
                if err != nil {
                {{ .Results.ReturnError $pkg "fmt.Errorf(\"failed to get user authentication: %w\", err)" }}
                }
            {{- else }}
                panic(err)
            {{- end }}
        {{- end }}


        {{ if gt (len .Results) 0 }} return {{ end }} m.getActive{{template "ReaderWriter" .HasAnnotation "@Write"}}().{{ .Name }}({{ range .Arguments }}{{ .Name }}, {{ end }})
        }
    {{- end }}
{{- end }}
