# {{ .name }}

{{ if not .feature }}
{{ sourceMarkdown .description }}
{{- else }}
{{ .description }}

_{{ .note }}_

### COMMUNITY FEATURE

{{- range .feature }}

**_{{ .name }}:_** {{ .text }}
{{- end }}
{{- end }}
