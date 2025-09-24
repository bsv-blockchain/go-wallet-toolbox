{{- $pkg := .Package }}
package {{ $pkg.Name }}

{{- if .Imports }}
    import (
    {{- range .Imports }}
        {{ . }}
    {{- end }}
    "fmt"
    "context"
    )
{{- end }}

{{ $pkg.OriginalPkgImportStatement }}

{{ $structName := "RPCStorageProvider" }}

{{/*Skip some methods and implement them by ourselves*/}}
{{- $skipMethods := strings "FindOrInsertUser"  }}

{{- with index .Interfaces 0 }}
    {{- range .Methods }}

        {{- if contains $skipMethods .Name }}
            {{ continue }}
        {{- end }}

        {{- range .Comments }}
            // {{ . }}
        {{- end}}
        func (p *{{ $structName }}) {{ .Name }}({{ range .Arguments }}{{ .Name }} {{ .Type | printType }}, {{ end }}) ({{ range .Results }}{{ .Type | printType }},{{ end }}) {
        {{- if .HasAnnotation "@NonRPC" }}
            {{- if .Results.HasError }}
                {{ .Results.ReturnError $pkg "fmt.Errorf(\"method not allowed to be called via RPC\")" }}
            {{- else }}
                panic("method not allowed to be called via RPC")
            {{- end }}
        {{- else }}
            {{- $authIDVarName := .Arguments.ArgumentOfType "AuthID" }}
            {{- $contextVarName := coalesce (.Arguments.ArgumentOfType "context.Context") "context.Background()" }}
            {{- if $authIDVarName }}
                err := p.verifyAuthID({{ $contextVarName }}, {{ $authIDVarName }})
            {{- else }}
                err := p.verifyAuthenticated({{ $contextVarName }})
            {{- end }}
            if err != nil {
            {{- if .Results.HasError }}
                {{ .Results.ReturnError $pkg "err" }}
            {{- else }}
                panic(err)
            {{- end }}
            }
            {{- if $authIDVarName }}
                err = p.ensureUserID({{ $contextVarName }}, &{{ $authIDVarName }})
                if err != nil {
                {{- if .Results.HasError }}
                    {{ .Results.ReturnError $pkg "err" }}
                {{- else }}
                    panic(err)
                {{- end }}
                }
            {{- end }}

            {{ if gt (len .Results) 0 }} return {{ end }} p.localProvider.{{ .Name }}({{ range .Arguments }}{{ .Name }}, {{ end }})
        {{- end }}
        }
    {{ end }}
{{- end }}
