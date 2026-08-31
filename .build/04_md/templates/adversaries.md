---
Tier: {{ .tier }}
Type: {{ yamlText .type }}
---

# {{ .name }}

**_Tier {{ .tier }} {{ .type }}._** _{{ .description }}_

- **Motives & Tactics:** {{ .motives_and_tactics }}
- **Difficulty:** {{ .difficulty }} | **Thresholds:** {{ .thresholds }} | **HP:** {{ .hp }} | **Stress:** {{ .stress }}
- **ATK:** {{ .atk }} | **{{ .attack }}:** {{ .range }} | {{ .damage }}{{ if .experience }}
- **Experience:** {{ .experience }}{{ end }}

### FEATURES

{{- range .feature }}

**_{{ .name }}:_** {{ mechanicsText (adversaryFeatureText .text) }}
{{- end }}
