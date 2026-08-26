# {{ .name }}

{{ .description }}

### TRANSFORMATION FEATURES

{{- range .feature }}

**_{{ .name }}:_** {{ mechanicsText .text }}
{{- end }}

### TRANSFORMATION QUESTIONS

{{- range .question }}

- {{ .text }}
{{- end }}
