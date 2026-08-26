# {{ .name }}

**_Tier {{ .tier }} {{ .type }}._**{{ if .description }} _{{ .description }}_{{ end }}

- **Impulses:** {{ .impulses }}
- **Difficulty:** {{ .difficulty }}
- **Potential Adversaries:** {{ environmentAdversaryLinks .potential_adversaries }}

### FEATURES

{{- range .feature }}

**_{{ .name }}:_** {{ .text }}{{ if .question }} _{{ .question }}_{{ end }}
{{- end }}
