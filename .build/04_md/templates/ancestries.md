# {{ .name }}

{{- if not .feature }}
{{ sourceMarkdown .description }}
{{- else }}
{{ .description }}

### ANCESTRY FEATURES

{{- range .feature }}

**_{{ .name }}:_** {{ .text }}
{{- end }}
{{- end }}
